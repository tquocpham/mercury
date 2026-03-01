import asyncio
import argparse
import json
from urllib.parse import urlencode

import httpx
import websockets
from textual.app import App, ComposeResult
from textual.containers import Vertical
from textual.widgets import Footer, Header, Input, RichLog


class ChatClient:
    def __init__(self, server_addr, convo_id):
        self.__server_addr = server_addr
        self.__convo_id = convo_id

    async def get_messages(self, page_size=10, next_token=None):
        params = {
            "conversation_id": self.__convo_id,
            "page_size": page_size,
        }
        if next_token:
            params["next_token"] = next_token
        query_string = urlencode(params)
        url = f"{self.__server_addr}/api/v1/messages?{query_string}"
        response = await httpx.AsyncClient().get(url)
        return response.json()

    async def refresh_messages(self, message_id):
        params = {
            "conversation_id": self.__convo_id,
            "message_id": message_id,
        }
        query_string = urlencode(params)
        url = f"{self.__server_addr}/api/v1/messages/refresh?{query_string}"
        response = await httpx.AsyncClient().get(url)
        return response.json()

    async def send_message(self, username, message):
        await httpx.AsyncClient().post(f'{self.__server_addr}/api/v1/messages', json={
            "conversation_id": self.__convo_id,
            "body": message,
            "user": username,
        })


class ChatApp(App):
    TITLE = "Chat Prototype"
    BINDINGS = [("q", "quit", "Quit")]

    def __init__(self, client: ChatClient, user: str, ws_addr: str, convo_id: str):
        super().__init__()
        self.__messages = []
        self.__client = client
        self.__user = user
        self.__ws_addr = ws_addr
        self.__convo_id = convo_id

    def compose(self) -> ComposeResult:
        yield Header()
        with Vertical():
            yield RichLog(id="chat_box", markup=True)
            yield Input(placeholder="Type here...", id="user_input")
        yield Footer()

    async def on_mount(self) -> None:
        self.query_one("#user_input").focus()
        msg_response = await self.__client.get_messages()
        if "Messages" not in msg_response:
            assert False, msg_response
        self.__messages = msg_response["Messages"]
        for m in reversed(self.__messages):
            self.display_message(m["user"], m["body"])

        asyncio.create_task(self._ws_listener())
        self.set_interval(30.0, self.poll_server)

    async def _ws_listener(self) -> None:
        url = f"{self.__ws_addr}/api/v1/ws"
        try:
            async with websockets.connect(url) as ws:
                await ws.send(json.dumps({
                    "channels": [f"conversation:{self.__convo_id}"]
                }))
                async for raw in ws:
                    try:
                        msg = json.loads(raw)
                        self.display_message(msg["user"], msg["message"])
                    except Exception:
                        pass
        except Exception as e:
            self.display_message("System", f"WebSocket disconnected: {e}")

    async def poll_server(self) -> None:
        if not self.__messages:
            return
        message_id = self.__messages[0]['message_id']
        try:
            msg_response = await self.__client.refresh_messages(message_id)
        except Exception as ex:
            self.display_message('Error', str(ex))
            return
        self.__messages = msg_response["Messages"] + self.__messages
        for m in msg_response["Messages"]:
            self.display_message(m["user"], m["body"])

    def display_message(self, user: str, message: str) -> None:
        chat_box = self.query_one("#chat_box", RichLog)
        color = "magenta" if user == "Remote" else "green"
        chat_box.write(f"[bold {color}]{user}:[/bold {color}] {message}")

    async def on_input_submitted(self, event: Input.Submitted) -> None:
        if event.value.strip():
            await self.__client.send_message(self.__user, event.value)
            self.display_message(f"(sent) {self.__user}", event.value)
            self.query_one("#user_input", Input).value = ""


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--user", "-u", help="username", required=True)
    parser.add_argument("--addr", "-a", help="chat server HTTP address",
                        default='http://localhost:9001',
                        required=False)
    parser.add_argument("--ws-addr", "-w", help="notifier WebSocket address",
                        default='ws://localhost:9004',
                        required=False)
    parser.add_argument("--convoid", "-c", help="chat conversation id",
                        default='abc123123',
                        required=False)
    args = parser.parse_args()

    client = ChatClient(args.addr, args.convoid)
    app = ChatApp(client, args.user, args.ws_addr, args.convoid)
    app.run()

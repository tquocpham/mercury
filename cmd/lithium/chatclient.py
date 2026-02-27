import argparse
from urllib.parse import urlencode

import requests
from textual.app import App, ComposeResult
from textual.containers import Vertical
from textual.widgets import Footer, Header, Input, RichLog


class ChatClient:
    def __init__(self, server_addr, convo_id):
        self.__server_addr = server_addr
        self.__convo_id = convo_id

    def get_messages(self, page_size=10, next_token=None):
        params = {
            "conversation_id": self.__convo_id,
            "page_size": page_size,
        }
        if next_token:
            params["next_token"] = next_token
        query_string = urlencode(params)
        url = f"{self.__server_addr}/messages?{query_string}"
        response = requests.get(url)
        return response.json()

    def refresh_messages(self, message_id):
        params = {
            "conversation_id": self.__convo_id,
            "message_id": message_id,
        }
        query_string = urlencode(params)
        url = f"{self.__server_addr}/messages/refresh?{query_string}"
        response = requests.get(url)
        return response.json()

    def send_message(self, username, message):
        response = requests.post(f'{self.__server_addr}/messages', json={
            "conversation_id": self.__convo_id,
            "body": message,
            "user": username,
        })


class ChatApp(App):
    TITLE = "Chat Prototype"
    BINDINGS = [("q", "quit", "Quit")]

    def __init__(self, lithium_client: ChatClient, user: str):
        super().__init__()
        self.__messages = []
        self.__lithium_client = lithium_client
        self.__user = user

    def compose(self) -> ComposeResult:
        yield Header()
        with Vertical():
            yield RichLog(id="chat_box", markup=True)
            yield Input(placeholder="Type here...", id="user_input")
        yield Footer()

    def on_mount(self) -> None:
        self.query_one("#user_input").focus()
        msg_response = self.__lithium_client.get_messages()
        if "Messages" not in msg_response:
            assert False, msg_response
        self.__messages = msg_response["Messages"]
        for m in reversed(self.__messages):
            self.display_message(m["user"], m["body"])

        # Start polling every 2 seconds
        # Using set_interval schedules the call on the UI loop
        self.set_interval(1.0, self.poll_server)

    async def poll_server(self) -> None:
        """Background task to check for new messages."""
        # In a real app, you would use: response = await httpx.AsyncClient().get(URL)
        # For this prototype, we'll simulate a 10% chance of a new message
        # self.display_message(
        #     "Remote", "This is a new message from the server!")
        message_id = self.__messages[0]['message_id']
        msg_response = self.__lithium_client.refresh_messages(message_id)
        self.__messages = msg_response["Messages"] + self.__messages
        for m in msg_response["Messages"]:
            self.display_message(m["user"], m["body"])

    def display_message(self, user: str, message: str) -> None:
        """Helper to write formatted messages to the log."""
        chat_box = self.query_one("#chat_box", RichLog)
        color = "magenta" if user == "Remote" else "green"
        chat_box.write(f"[bold {color}]{user}:[/bold {color}] {message}")

    async def on_input_submitted(self, event: Input.Submitted) -> None:
        if event.value.strip():
            self.__lithium_client.send_message(self.__user, event.value)
            self.display_message(f"(sent) {self.__user}", event.value)
            self.query_one("#user_input", Input).value = ""


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--user", "-u", help="username", required=True)
    parser.add_argument("--addr", "-a", help="chat_server_addr",
                        default='http://localhost:8080',
                        required=False)
    parser.add_argument("--convoid", "-c", help="chat_conversation_id",
                        default='abc123123',
                        required=False)
    args = parser.parse_args()
    user = args.user
    server_addr = args.addr
    convid = args.convoid

    lithium_client = ChatClient(server_addr, convid)
    app = ChatApp(lithium_client, user)
    app.run()

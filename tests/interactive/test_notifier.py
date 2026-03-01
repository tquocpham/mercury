import asyncio
import sys
import time
import json

import websockets


WS_URL = "ws://localhost:9004/api/v1/ws"


async def test_sub(ws, channel: str) -> bool:
    await ws.send(json.dumps({
        "channels": [channel],
    }))
    print(
        f"Subscribed to channel: {channel}\nWaiting for messages...\n")
    # block forever, print each message as it arrives
    async for message in ws:
        print(f"Received: {message}")

    # response = await asyncio.wait_for(ws.recv(), timeout=5.0)
    # print(response)


async def run_tests():
    while True:
        print(f"Connecting to {WS_URL} ...")
        try:
            async with websockets.connect(WS_URL) as ws:
                print("Connected.\n")
                await test_sub(ws, "conversation:abc123123")

        except OSError as e:
            print(f"Connection failed: {e}")
            print(f"Is the websocket server running at {WS_URL}?")
            print(f'sleeping 5s. trying again')
            time.sleep(5)
            continue
        except websockets.exceptions.ConnectionClosedError as e:
            print(f"Connection failed: {e}")
            print(f'sleeping 5s. trying again')
            time.sleep(5)
            continue
        except Exception as e:
            print(f"Connection failed: {e}")
            return False


async def main():
    await run_tests()

if __name__ == "__main__":
    asyncio.run(main())

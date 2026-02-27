import requests
import json

messages = []
addr = 'http://localhost:8080'
convid = "abc123123"

username = input("who are you?: ")
print(f'loading recent chat data...')
response = requests.get(
    f'{addr}/messages?conversation_id={convid}&page_size=2')

print(json.dumps(response.json(), indent=2))

response = requests.post(f'{addr}/messages', json={
    "conversation_id": convid,
    "body": "hello world",
    "user": username,
})

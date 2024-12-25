
import pytest
import requests
import json
import time
import uuid
import logging
from dataclasses import dataclass
from typing import Dict, List
from websocket import create_connection, WebSocketConnectionClosedException

# Configure logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

@dataclass
class TestUser:
    username: str
    password: str
    name: str
    token: str = None
    user_id: int = None

class TestConfig:
    BASE_URL = "http://localhost:8080/api"
    WS_URL = "ws://localhost:8080/api/ws"

class TestBase:
    @staticmethod
    def make_request(method: str, endpoint: str, **kwargs) -> requests.Response:
        """Helper method to make HTTP requests with logging"""
        url = f"{TestConfig.BASE_URL}/{endpoint}"
        logger.info(f"Making {method} request to {url}")
        try:
            response = requests.request(method, url, **kwargs)
            logger.info(f"Response status: {response.status_code}")
            if response.status_code >= 400:
                logger.error(f"Error response: {response.text}")
            return response
        except Exception as e:
            logger.error(f"Request failed: {str(e)}")
            raise

# Fixtures
@pytest.fixture(scope="session")
def test_users() -> List[TestUser]:
    """Create unique test users"""
    unique_suffix = str(uuid.uuid4())[:8]
    return [
        TestUser(
            username=f"testuser_{unique_suffix}",
            password="testpass123",
            name="Test User"
        ),
        TestUser(
            username=f"seconduser_{unique_suffix}",
            password="secondpass123",
            name="Second User"
        )
    ]

@pytest.fixture(scope="session")
def registered_users(test_users) -> List[TestUser]:
    """Register test users and return their data"""
    for user in test_users:
        response = TestBase.make_request(
            "post",
            "register",
            json={
                "username": user.username,
                "password": user.password,
                "name": user.name
            }
        )
        assert response.status_code == 200, f"Failed to register user {user.username}: {response.text}"
        logger.info(f"User {user.username} registered successfully")
    return test_users

@pytest.fixture(scope="session")
def authenticated_users(registered_users) -> List[TestUser]:
    """Login test users and store their tokens"""
    for user in registered_users:
        response = TestBase.make_request(
            "post",
            "login",
            json={
                "username": user.username,
                "password": user.password
            }
        )
        assert response.status_code == 200, f"Failed to login user {user.username}: {response.text}"
        user.token = response.json()["token"]
        logger.info(f"User {user.username} authenticated successfully")
    return registered_users

@pytest.fixture(scope="session")
def users_with_ids(authenticated_users) -> List[TestUser]:
    """Fetch user IDs for authenticated users"""
    headers = {"Authorization": f"Bearer {authenticated_users[0].token}"}
    for user in authenticated_users:
        response = TestBase.make_request(
            "get",
            "search",
            params={"username": user.username},
            headers=headers
        )
        assert response.status_code == 200, f"Failed to get ID for user {user.username}: {response.text}"
        users = response.json().get("users", [])
        assert len(users) == 1, f"Ambiguous search results for user {user.username}"
        user.user_id = users[0]["id"]
        logger.info(f"User {user.username} has ID {user.user_id}")
    return authenticated_users

@pytest.fixture(scope="session")
def websocket_connections(authenticated_users):
    """Establish WebSocket connections for users"""
    connections = {}
    for user in authenticated_users:
        try:
            ws = create_connection(
                TestConfig.WS_URL,
                header=[f"Authorization: Bearer {user.token}"]
            )
            connections[user.username] = ws
            logger.info(f"WebSocket connection established for {user.username}")
        except Exception as e:
            logger.error(f"Failed to establish WebSocket connection for {user.username}: {e}")
            raise
    yield connections
    for username, ws in connections.items():
        ws.close()
        logger.info(f"WebSocket connection closed for {username}")

class TestAuthentication:
    def test_register_new_user(self):
        """Test user registration with unique credentials"""
        unique_suffix = str(uuid.uuid4())[:8]
        user_data = {
            "username": f"uniqueuser_{unique_suffix}",
            "password": "uniquepass123",
            "name": "Unique User"
        }
        
        # Test successful registration
        response = TestBase.make_request("post", "register", json=user_data)
        assert response.status_code == 200
        assert response.json()["message"] == "Пользователь успешно зарегистрирован"
        
        # Test duplicate registration
        response = TestBase.make_request("post", "register", json=user_data)
        assert response.status_code == 400
        assert response.json()["error"] == "Имя пользователя уже занято"

    def test_login_validation(self, authenticated_users):
        """Test login with valid and invalid credentials"""
        user = authenticated_users[0]
        
        # Test invalid password
        response = TestBase.make_request(
            "post",
            "login",
            json={
                "username": user.username,
                "password": "wrongpassword"
            }
        )
        assert response.status_code == 401
        assert response.json()["error"] == "Неверные учетные данные"

class TestUserProfile:
    def test_profile_access(self, users_with_ids):
        """Test profile retrieval and privacy"""
        user = users_with_ids[0]
        headers = {"Authorization": f"Bearer {user.token}"}
        
        response = TestBase.make_request(
            "get",
            f"profile/{user.user_id}",
            headers=headers
        )
        assert response.status_code == 200
        profile = response.json()["user"]
        assert profile["username"] == user.username
        assert profile["name"] == user.name
        assert "password" not in profile

class TestChat:
    def test_chat_lifecycle(self, users_with_ids):
        """Test chat creation and retrieval"""
        user1, user2 = users_with_ids[:2]
        headers = {"Authorization": f"Bearer {user1.token}"}
        
        # Create chat
        chat_data = {
            "type": "personal",
            "user_ids": [user1.user_id, user2.user_id]
        }
        response = TestBase.make_request(
            "post",
            "create-chat",
            json=chat_data,
            headers=headers
        )
        assert response.status_code == 200
        chat = response.json()["chat"]
        assert chat["type"] == "personal"
        assert len(chat["users"]) == 2
        
        # Verify chat appears in list
        response = TestBase.make_request("get", "chats", headers=headers)
        assert response.status_code == 200
        assert any(c["id"] == chat["id"] for c in response.json()["chats"])

class TestMessaging:
    def test_message_operations(self, users_with_ids):
        """Test message sending, retrieval, and deletion"""
        sender, receiver = users_with_ids[:2]
        headers = {"Authorization": f"Bearer {sender.token}"}
        
        # Send message
        message_data = {
            "receiver_id": receiver.user_id,
            "content": "Test message content",
            "media_url": ""
        }
        response = TestBase.make_request(
            "post",
            "message",
            json=message_data,
            headers=headers
        )
        assert response.status_code == 200
        message_id = response.json()["message_id"]
        
        # Verify message delivery
        time.sleep(1)  # Allow for message processing
        response = TestBase.make_request("get", "messages", headers=headers)
        assert response.status_code == 200
        messages = response.json()["messages"]
        assert any(m["id"] == message_id for m in messages)
        
        # Delete message
        response = TestBase.make_request(
            "delete",
            f"delete-message/{message_id}",
            headers=headers
        )
        assert response.status_code == 200
        
        # Verify deletion
        response = TestBase.make_request("get", "messages", headers=headers)
        assert response.status_code == 200
        assert not any(m["id"] == message_id for m in response.json()["messages"])

class TestUserBlocking:
    def test_blocking_functionality(self, users_with_ids):
        """Test user blocking and message restrictions"""
        blocker, blocked = users_with_ids[:2]
        headers = {"Authorization": f"Bearer {blocker.token}"}
        
        # Block user
        response = TestBase.make_request(
            "post",
            "block-user",
            json={"block_id": blocked.user_id},
            headers=headers
        )
        assert response.status_code == 200
        
        # Verify blocked user cannot send messages
        headers_blocked = {"Authorization": f"Bearer {blocked.token}"}
        response = TestBase.make_request(
            "post",
            "message",
            json={
                "receiver_id": blocker.user_id,
                "content": "Should not be delivered",
                "media_url": ""
            },
            headers=headers_blocked
        )
        assert response.status_code == 403
        assert response.json()["error"] == "Вы заблокированы этим пользователем"

class TestWebSocket:
    def test_websocket_messaging(self, users_with_ids, websocket_connections):
        """Test real-time messaging via WebSocket"""
        sender, receiver = users_with_ids[:2]
        sender_ws = websocket_connections[sender.username]
        receiver_ws = websocket_connections[receiver.username]
        
        # Send message
        message = {
            "receiver_id": receiver.user_id,
            "content": "WebSocket test message",
            "media_url": ""
        }
        sender_ws.send(json.dumps(message))
        
        # Verify receipt
        try:
            receiver_ws.settimeout(5)
            response = json.loads(receiver_ws.recv())
            assert response["content"] == "WebSocket test message"
            assert response["sender_id"] == sender.user_id
            assert response["receiver_id"] == receiver.user_id
        except Exception as e:
            pytest.fail(f"WebSocket message verification failed: {e}")

    def test_websocket_blocking(self, users_with_ids, websocket_connections):
        """Test WebSocket messaging with blocked user"""
        blocker, blocked = users_with_ids[:2]
        blocker_ws = websocket_connections[blocker.username]
        blocked_ws = websocket_connections[blocked.username]
        
        # Block user
        headers = {"Authorization": f"Bearer {blocker.token}"}
        TestBase.make_request(
            "post",
            "block-user",
            json={"block_id": blocked.user_id},
            headers=headers
        )
        
        # Attempt message from blocked user
        message = {
            "receiver_id": blocker.user_id,
            "content": "Should be blocked",
            "media_url": ""
        }
        blocked_ws.send(json.dumps(message))
        
        # Verify message blocking
        try:
            blocker_ws.settimeout(5)
            blocker_ws.recv()
            pytest.fail("Blocked user's message was received")
        except WebSocketConnectionClosedException:
            pass  # Expected behavior
        except Exception as e:
            if "timed out" in str(e):
                pass  # Also acceptable
            else:
                raise

if __name__ == "__main__":
    pytest.main(["-v", __file__])

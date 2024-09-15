import unittest
import requests
import jwt

# Константы для работы с API
BASE_URL = "http://localhost:8080"

# Данные пользователей для регистрации
TEST_USERS = [
    {
        "username": "test_user1",
        "email": "test_user1@example.com",
        "password": "password123"
    },
    {
        "username": "test_user2",
        "email": "test_user2@example.com",
        "password": "password456"
    }
]

class TestAPI(unittest.TestCase):

    @classmethod
    def setUpClass(cls):
        """Метод выполняется один раз перед запуском всех тестов"""
        cls.user1_token, cls.user1_id = cls.authenticate(TEST_USERS[0])
        cls.user2_token, cls.user2_id = cls.authenticate(TEST_USERS[1])
        print(f"user1_id: {cls.user1_id}")
        print(f"user2_id: {cls.user2_id}")

    @staticmethod
    def authenticate(user):
        """Метод для аутентификации пользователя и получения JWT токена"""
        response = requests.post(f"{BASE_URL}/login", json={"email": user["email"], "password": user["password"]})
        if response.status_code == 200:
            token = response.json().get('token')
            print(f"Authenticated {user['username']} successfully. Token: {token}")
            # Проверка, что токен не пустой и имеет правильный формат
            if not token or not TestAPI.is_jwt_token_valid(token):
                raise Exception(f"Invalid token received for {user['username']}. Token: {token}")
            # Декодируем токен, чтобы получить user_id
            decoded = jwt.decode(token, options={"verify_signature": False}, algorithms=["HS256"])
            user_id = decoded.get('user_id')
            if not user_id:
                raise Exception(f"user_id not found in token for {user['username']}. Token: {token}")
            return token, user_id
        else:
            raise Exception(f"Failed to authenticate {user['username']}. Status code: {response.status_code}, Error: {response.text}")

    @staticmethod
    def is_jwt_token_valid(token):
        """Проверяет, является ли токен действительным JWT"""
        try:
            # Попробуем декодировать токен без проверки подписи, чтобы проверить его формат
            jwt.decode(token, options={"verify_signature": False}, algorithms=["HS256"])
            return True
        except jwt.DecodeError:
            return False

    def test_get_user_info(self):
        """Тест получения информации о пользователе"""
        headers = {"Authorization": f"Bearer {self.user1_token}"}
        response = requests.get(f"{BASE_URL}/users/{self.user1_id}", headers=headers)
        self.assertEqual(response.status_code, 200, f"Failed to get user info. Status code: {response.status_code}, Error: {response.text}")

    def test_send_text_message(self):
        """Тест отправки текстового сообщения"""
        headers = {"Authorization": f"Bearer {self.user1_token}"}
        payload = {
            "sender_id": self.user1_id,
            "receiver_id": self.user2_id,
            "content": "Hello from test!"
        }
        response = requests.post(f"{BASE_URL}/messages/text", json=payload, headers=headers)
        self.assertEqual(response.status_code, 200, f"Failed to send text message. Status code: {response.status_code}, Error: {response.text}")

    def test_get_text_messages(self):
        """Тест получения текстовых сообщений между пользователями"""
        headers = {"Authorization": f"Bearer {self.user1_token}"}
        response = requests.get(f"{BASE_URL}/messages/text?sender_id={self.user1_id}&receiver_id={self.user2_id}", headers=headers)
        self.assertEqual(response.status_code, 200, f"Failed to get text messages. Status code: {response.status_code}, Error: {response.text}")

    def test_deactivate_user(self):
        """Тест деактивации пользователя"""
        headers = {"Authorization": f"Bearer {self.user1_token}"}
        response = requests.post(f"{BASE_URL}/users/{self.user1_id}/deactivate", headers=headers)
        self.assertEqual(response.status_code, 200, f"Failed to deactivate user. Status code: {response.status_code}, Error: {response.text}")

    def test_activate_user(self):
        """Тест активации пользователя"""
        headers = {"Authorization": f"Bearer {self.user1_token}"}
        response = requests.post(f"{BASE_URL}/users/{self.user1_id}/activate", headers=headers)
        self.assertEqual(response.status_code, 200, f"Failed to activate user. Status code: {response.status_code}, Error: {response.text}")

if __name__ == "__main__":
    unittest.main()

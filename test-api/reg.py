import requests

# Константы для API
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

def register_user(user):
    """Регистрация пользователя через API."""
    response = requests.post(f"{BASE_URL}/users", json=user)
    
    if response.status_code == 201:
        print(f"User {user['username']} registered successfully.")
    elif response.status_code == 409:
        print(f"User {user['username']} already exists.")
    else:
        print(f"Failed to register user {user['username']}. Status code: {response.status_code}, Error: {response.text}")

def main():
    """Регистрация всех тестовых пользователей."""
    for user in TEST_USERS:
        register_user(user)

if __name__ == "__main__":
    main()

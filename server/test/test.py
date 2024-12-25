# test_app.py

import pytest
import requests
import json
import time
import uuid
from websocket import create_connection, WebSocketConnectionClosedException

# Конфигурация
BASE_URL = "http://localhost:8080/api"
WS_URL = "ws://localhost:8080/api/ws"

# Фикстуры для pytest
@pytest.fixture(scope="session")
def register_test_users():
    """
    Регистрирует двух уникальных пользователей для тестирования.
    Возвращает список с данными пользователей.
    """
    unique_suffix = str(uuid.uuid4())[:8]
    users = [
        {
            "username": f"testuser_{unique_suffix}",
            "password": "testpass123",
            "name": "Test User"
        },
        {
            "username": f"seconduser_{unique_suffix}",
            "password": "secondpass123",
            "name": "Second User"
        }
    ]
    registered_users = []
    for user in users:
        response = requests.post(f"{BASE_URL}/register", json=user)
        if response.status_code == 200:
            print(f"Пользователь {user['username']} успешно зарегистрирован.")
        elif response.status_code == 400 and response.json().get("error") == "Имя пользователя уже занято":
            pytest.fail(f"Пользователь {user['username']} уже существует.")
        else:
            pytest.fail(f"Не удалось зарегистрировать пользователя {user['username']}: {response.text}")
        registered_users.append(user)
    return registered_users

@pytest.fixture(scope="session")
def login_test_users(register_test_users):
    """
    Авторизует зарегистрированных пользователей и возвращает их JWT-токены.
    """
    tokens = {}
    for user in register_test_users:
        login_data = {
            "username": user["username"],
            "password": user["password"]
        }
        response = requests.post(f"{BASE_URL}/login", json=login_data)
        if response.status_code == 200:
            tokens[user["username"]] = response.json()["token"]
            print(f"Пользователь {user['username']} успешно авторизован.")
        else:
            pytest.fail(f"Не удалось авторизовать пользователя {user['username']}: {response.text}")
    return tokens

@pytest.fixture(scope="session")
def get_user_ids(register_test_users, login_test_users):
    """
    Получает ID зарегистрированных пользователей.
    Возвращает словарь с именами пользователей и их ID.
    """
    user_ids = {}
    # Используем токен первого пользователя для аутентификации
    first_username = register_test_users[0]["username"]
    token = login_test_users[first_username]
    headers = {"Authorization": f"Bearer {token}"}

    for user in register_test_users:
        print(f"Making GET /search for username: {user['username']} with headers: {headers}")
        response = requests.get(f"{BASE_URL}/search", params={"username": user["username"]}, headers=headers)
        print(f"Response status: {response.status_code}, body: {response.text}")
        if response.status_code == 200:
            users = response.json().get("users", [])
            if len(users) == 1:
                user_ids[user["username"]] = users[0]["id"]
                print(f"Пользователь {user['username']} имеет ID {users[0]['id']}.")
            else:
                pytest.fail(f"Не удалось однозначно найти пользователя {user['username']}.")
        else:
            pytest.fail(f"Ошибка при поиске пользователя {user['username']}: {response.text}")
    return user_ids

@pytest.fixture(scope="session")
def websocket_connections(login_test_users):
    """
    Устанавливает WebSocket-соединения для двух пользователей.
    Возвращает словарь с объектами WebSocket.
    """
    connections = {}
    for username, token in login_test_users.items():
        try:
            # Передаём заголовок Authorization корректно
            ws = create_connection(WS_URL, header=[f"Authorization: Bearer {token}"])
            connections[username] = ws
            print(f"WebSocket-соединение для пользователя {username} установлено.")
        except Exception as e:
            pytest.fail(f"Не удалось установить WebSocket-соединение для пользователя {username}: {e}")
    yield connections
    # Закрытие соединений после тестов
    for username, ws in connections.items():
        ws.close()
        print(f"WebSocket-соединение для пользователя {username} закрыто.")

# Тесты API

def test_register_user():
    """Тестирует регистрацию нового пользователя."""
    unique_suffix = str(uuid.uuid4())[:8]
    user_data = {
        "username": f"uniqueuser_{unique_suffix}",
        "password": "uniquepass123",
        "name": "Unique User"
    }
    response = requests.post(f"{BASE_URL}/register", json=user_data)
    assert response.status_code == 200, f"Регистрация не удалась: {response.text}"
    assert response.json()["message"] == "Пользователь успешно зарегистрирован"

    # Попытка зарегистрировать того же пользователя снова должна завершиться ошибкой
    response_duplicate = requests.post(f"{BASE_URL}/register", json=user_data)
    assert response_duplicate.status_code == 400, "Дублирующая регистрация должна завершиться ошибкой"
    assert response_duplicate.json()["error"] == "Имя пользователя уже занято"

def test_login_user(login_test_users, get_user_ids):
    """Тестирует авторизацию зарегистрированных пользователей."""
    for username, token in login_test_users.items():
        assert isinstance(token, str) and len(token) > 0, f"Токен для пользователя {username} некорректен."

    # Попытка авторизации с неверным паролем
    # Генерируем новый уникальный username для теста неверного пароля
    unique_suffix = str(uuid.uuid4())[:8]
    username = f"wrongpassuser_{unique_suffix}"
    user_data = {
        "username": username,
        "password": "correctpassword",
        "name": "Wrong Pass User"
    }
    response_register = requests.post(f"{BASE_URL}/register", json=user_data)
    assert response_register.status_code == 200, f"Не удалось зарегистрировать пользователя: {response_register.text}"

    login_data_wrong = {
        "username": username,
        "password": "wrongpassword"
    }
    response_wrong = requests.post(f"{BASE_URL}/login", json=login_data_wrong)
    assert response_wrong.status_code == 401, "Авторизация с неверным паролем должна завершиться ошибкой"
    assert response_wrong.json()["error"] == "Неверные учетные данные"

def test_get_profile(login_test_users, get_user_ids):
    """Тестирует получение профиля текущего пользователя."""
    for username, token in login_test_users.items():
        user_id = get_user_ids[username]
        headers = {"Authorization": f"Bearer {token}"}
        response = requests.get(f"{BASE_URL}/profile/{user_id}", headers=headers)
        assert response.status_code == 200, f"Не удалось получить профиль: {response.text}"
        user = response.json()["user"]
        assert user["username"] == username
        assert user["name"] == user["name"]
        assert "password" not in user  # Пароль не должен возвращаться

def test_create_and_get_chats(login_test_users, get_user_ids):
    """Тестирует создание нового чата и получение списка чатов."""
    for username, token in login_test_users.items():
        user_id = get_user_ids[username]
        headers = {"Authorization": f"Bearer {token}"}
        # Определяем второго пользователя
        suffix = username.split("_")[-1]
        other_username = f"seconduser_{suffix}"
        if other_username not in get_user_ids:
            pytest.fail(f"Пользователь {other_username} не найден.")
        other_user_id = get_user_ids[other_username]
        
        chat_data = {
            "type": "personal",
            "user_ids": [user_id, other_user_id]
        }
        response_create = requests.post(f"{BASE_URL}/create-chat", json=chat_data, headers=headers)
        assert response_create.status_code == 200, f"Не удалось создать чат: {response_create.text}"
        assert response_create.json()["message"] == "Чат создан"
        chat = response_create.json()["chat"]
        assert chat["type"] == "personal"
        assert len(chat["users"]) == 2

        response_get = requests.get(f"{BASE_URL}/chats", headers=headers)
        assert response_get.status_code == 200, f"Не удалось получить чаты: {response_get.text}"
        chats = response_get.json()["chats"]
        assert any(c["id"] == chat["id"] for c in chats), "Созданный чат отсутствует в списке чатов"

def test_send_and_get_messages(login_test_users, get_user_ids):
    """Тестирует отправку сообщения и получение списка сообщений."""
    for username, token in login_test_users.items():
        user_id = get_user_ids[username]
        # Определяем второго пользователя
        suffix = username.split("_")[-1]
        other_username = f"seconduser_{suffix}"
        other_user_id = get_user_ids.get(other_username)
        if not other_user_id:
            pytest.fail(f"Пользователь {other_username} не найден.")
        headers = {"Authorization": f"Bearer {token}"}
        message_content = "Hello, this is a test message!"
        message_data = {
            "receiver_id": other_user_id,
            "content": message_content,
            "media_url": ""
        }
        response_send = requests.post(f"{BASE_URL}/message", json=message_data, headers=headers)
        assert response_send.status_code == 200, f"Не удалось отправить сообщение: {response_send.text}"
        assert response_send.json()["message"] == "Сообщение отправлено"
        message_id = response_send.json()["message_id"]

        # Даем серверу немного времени для обработки сообщения
        time.sleep(1)

        response_get = requests.get(f"{BASE_URL}/messages", headers=headers)
        if response_get.status_code != 200:
            pytest.fail(f"Не удалось получить сообщения: {response_get.text}")
        messages = response_get.json()["messages"]
        assert any(m["id"] == message_id and m["content"] == message_content for m in messages), "Отправленное сообщение отсутствует в списке сообщений"

def test_block_user(login_test_users, get_user_ids):
    """Тестирует блокировку другого пользователя."""
    for username, token in login_test_users.items():
        user_id = get_user_ids[username]
        # Определяем второго пользователя
        suffix = username.split("_")[-1]
        other_username = f"seconduser_{suffix}"
        other_user_id = get_user_ids.get(other_username)
        if not other_user_id:
            pytest.fail(f"Пользователь {other_username} не найден.")
        headers = {"Authorization": f"Bearer {token}"}
        block_data = {
            "block_id": other_user_id
        }
        response_block = requests.post(f"{BASE_URL}/block-user", json=block_data, headers=headers)
        assert response_block.status_code == 200, f"Не удалось заблокировать пользователя: {response_block.text}"
        assert response_block.json()["message"] == "Пользователь заблокирован"

        # Попытка отправить сообщение заблокированному пользователю должна завершиться ошибкой
        message_data = {
            "receiver_id": other_user_id,
            "content": "This message should not be sent.",
            "media_url": ""
        }
        response_send = requests.post(f"{BASE_URL}/message", json=message_data, headers=headers)
        assert response_send.status_code == 403, "Отправка сообщения заблокированному пользователю должна завершиться ошибкой"
        assert response_send.json()["error"] == "Вы заблокированы этим пользователем"

def test_upload_file(login_test_users, get_user_ids):
    """Тестирует загрузку файла на сервер."""
    for username, token in login_test_users.items():
        user_id = get_user_ids[username]
        headers = {"Authorization": f"Bearer {token}"}
        files = {
            'file': ('test.txt', 'This is a test file.')
        }
        response_upload = requests.post(f"{BASE_URL}/upload", files=files, headers=headers)
        assert response_upload.status_code == 200, f"Не удалось загрузить файл: {response_upload.text}"
        assert response_upload.json()["message"] == "Файл успешно загружен"
        assert "file_url" in response_upload.json()

def test_delete_message(login_test_users, get_user_ids):
    """Тестирует удаление отправленного сообщения."""
    for username, token in login_test_users.items():
        user_id = get_user_ids[username]
        # Определяем второго пользователя
        suffix = username.split("_")[-1]
        other_username = f"seconduser_{suffix}"
        other_user_id = get_user_ids.get(other_username)
        if not other_user_id:
            pytest.fail(f"Пользователь {other_username} не найден.")
        headers = {"Authorization": f"Bearer {token}"}
        # Сначала отправляем сообщение
        message_data = {
            "receiver_id": other_user_id,
            "content": "Message to be deleted.",
            "media_url": ""
        }
        response_send = requests.post(f"{BASE_URL}/message", json=message_data, headers=headers)
        assert response_send.status_code == 200, f"Не удалось отправить сообщение: {response_send.text}"
        message_id = response_send.json()["message_id"]

        # Даем серверу немного времени для обработки сообщения
        time.sleep(1)

        # Удаляем сообщение
        response_delete = requests.delete(f"{BASE_URL}/delete-message/{message_id}", headers=headers)
        assert response_delete.status_code == 200, f"Не удалось удалить сообщение: {response_delete.text}"
        assert response_delete.json()["message"] == "Сообщение успешно удалено"

        # Проверяем, что сообщение отсутствует в списке
        response_get = requests.get(f"{BASE_URL}/messages", headers=headers)
        if response_get.status_code != 200:
            pytest.fail(f"Не удалось получить сообщения: {response_get.text}")
        messages = response_get.json()["messages"]
        assert not any(m["id"] == message_id for m in messages), "Удалённое сообщение всё ещё присутствует в списке сообщений"

# Тесты WebSocket

def test_websocket_send_and_receive(websocket_connections, get_user_ids):
    """
    Тестирует отправку и получение сообщения через WebSocket.
    """
    # Определяем пользователей
    usernames = list(websocket_connections.keys())
    if len(usernames) < 2:
        pytest.fail("Необходимо как минимум два пользователя для теста WebSocket.")
    sender_username = usernames[0]
    receiver_username = usernames[1]
    sender_ws = websocket_connections[sender_username]
    receiver_ws = websocket_connections[receiver_username]
    sender_id = get_user_ids[sender_username]
    receiver_id = get_user_ids[receiver_username]

    # Отправляем сообщение от sender к receiver
    message = {
        "receiver_id": receiver_id,
        "content": "Hello via WebSocket!",
        "media_url": ""
    }
    sender_ws.send(json.dumps(message))
    print("Сообщение отправлено через WebSocket.")

    # Ожидаем сообщение на стороне receiver
    try:
        result = receiver_ws.recv()
        response = json.loads(result)
        assert response["content"] == "Hello via WebSocket!", "Полученное сообщение не соответствует отправленному."
        assert response["sender_id"] == sender_id, "ID отправителя не соответствует."
        assert response["receiver_id"] == receiver_id, "ID получателя не соответствует."
    except WebSocketConnectionClosedException:
        pytest.fail("WebSocket-соединение было закрыто неожиданно.")
    except Exception as e:
        pytest.fail(f"Ошибка при получении сообщения через WebSocket: {e}")

def test_websocket_blocked_user(login_test_users, get_user_ids, websocket_connections):
    """
    Тестирует, что заблокированный пользователь не может отправлять сообщения через WebSocket.
    """
    # Определяем пользователей
    usernames = list(login_test_users.keys())
    if len(usernames) < 2:
        pytest.fail("Необходимо как минимум два пользователя для теста WebSocket.")
    blocker_username = usernames[0]
    blocked_username = usernames[1]
    blocker_id = get_user_ids[blocker_username]
    blocked_id = get_user_ids[blocked_username]
    blocker_token = login_test_users[blocker_username]
    blocked_token = login_test_users[blocked_username]
    blocker_ws = websocket_connections[blocker_username]
    blocked_ws = websocket_connections[blocked_username]

    # Заблокируйте blocked_username от blocker_username через API
    headers = {"Authorization": f"Bearer {blocker_token}"}
    block_data = {
        "block_id": blocked_id
    }
    response_block = requests.post(f"{BASE_URL}/block-user", json=block_data, headers=headers)
    assert response_block.status_code == 200, f"Не удалось заблокировать пользователя: {response_block.text}"
    assert response_block.json()["message"] == "Пользователь заблокирован"

    # Попытка отправить сообщение от blocked_username к blocker_username через WebSocket
    message = {
        "receiver_id": blocker_id,
        "content": "This message should be blocked.",
        "media_url": ""
    }
    blocked_ws.send(json.dumps(message))
    print("Заблокированный пользователь попытался отправить сообщение через WebSocket.")

    # Ожидаем, что blocker_username не получит сообщение
    blocker_ws.settimeout(5)  # Устанавливаем таймаут для ожидания
    try:
        result = blocker_ws.recv()
        response = json.loads(result)
        # Если сообщение получено, то тест не прошёл
        assert False, "Получено сообщение от заблокированного пользователя, хотя это не должно происходить."
    except Exception as e:
        # Ожидаем, что сообщение не будет получено в течение таймаута
        print("Сообщение от заблокированного пользователя не получено, как и ожидалось.")

# Запуск тестов
if __name__ == "__main__":
    pytest.main(["-v", "test_app.py"])

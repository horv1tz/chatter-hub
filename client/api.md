
| Метод | Эндпоинт | Протокол | Описание | Параметры (тело запроса) | Ответ (успех) | Ответ (ошибка) |
|-------|----------|----------|----------|-------------------------|---------------|----------------|
| POST  | /api/register      | HTTPS | Регистрация нового пользователя | { "username": "string", "password": "string", "name": "string" } | { "message": "Пользователь успешно зарегистрирован" } | 400 Bad Request (неверный формат) |
| POST  | /api/login         | HTTPS | Аутентификация пользователя | { "username": "string", "password": "string" } | { "message": "Успешный вход", "token": "string" } | 401 Unauthorized (неверные учетные данные) |
| GET   | /api/profile/:user_id | HTTPS | Получение профиля пользователя | - | { "user": { "id": "uint", "username": "string", "name": "string" } } | 404 Not Found (пользователь не найден) |
| GET   | /api/chats         | HTTPS | Получение списка чатов пользователя | - | { "chats": [ { "id": "uint", "type": "string", "name": "string" } ] } | 500 Internal Server Error |
| GET   | /api/search        | HTTPS | Поиск пользователей по имени | username (query параметр) | { "users": [ { "id": "uint", "username": "string", "name": "string" } ] } | 400 Bad Request (параметр отсутствует) |
| GET   | /api/ws            | WSS    | Установка WebSocket соединения | - | - | 401 Unauthorized (неверный токен) |
| POST  | /api/message       | HTTPS | Отправка сообщения | { "receiver_id": "uint", "content": "string", "media_url": "string" } | { "message": "Сообщение отправлено", "message_id": "uint" } | 400 Bad Request (неверный формат) |
| POST  | /api/create-chat   | HTTPS | Создание нового чата | { "type": "string", "name": "string", "user_ids": [ "uint" ] } | { "message": "Чат создан", "chat": { "id": "uint", "type": "string", "name": "string" } } | 400 Bad Request (неверный формат) |
| POST  | /api/block-user    | HTTPS | Блокировка пользователя | { "block_id": "uint" } | { "message": "Пользователь заблокирован" } | 400 Bad Request (пользователь не найден) |
| POST  | /api/upload        | HTTPS | Загрузка файла | file (multipart/form-data) | { "message": "Файл успешно загружен", "file_url": "string" } | 400 Bad Request (ошибка загрузки) |
| DELETE| /api/delete-message/:message_id | HTTPS | Удаление сообщения | - | { "message": "Сообщение успешно удалено" } | 404 Not Found (сообщение не найдено) |
| GET   | /api/messages      | HTTPS | Получение списка сообщений | - | { "messages": [ { "id": "uint", "content": "string", "timestamp": "time" } ] } | 500 Internal Server Error |
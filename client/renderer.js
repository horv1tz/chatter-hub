// renderer.js

// URL вашего API
const API_URL = 'http://localhost:8080';

// Переменные для хранения токена и ID пользователя
let token = '';
let userId = '';

// Переменная для WebSocket-соединения
let socket = null;

// Переменная для текущего выбранного чата
let currentChat = null;

window.api.getToken()
    .then(tokenData => {
        console.log('Токен и userId:', tokenData);
        token = tokenData.token;
        userId = tokenData.userId;
    })
    .catch(error => {
        console.error('Ошибка получения токена:', error);
    });


// При загрузке DOM выполняем инициализацию
window.addEventListener('DOMContentLoaded', async () => {
    try {
        // Получаем токен и userId из главного процесса
        const tokenData = await window.api.getToken();
        token = tokenData.token || '';
        userId = tokenData.userId || '';

        console.log('DOMContentLoaded');
        console.log('Token:', token);
        console.log('User ID:', userId);

        if (token) {
            // Если токен существует, загружаем чат и инициализируем WebSocket
            loadView('chat');
            initializeWebSocket();
        } else {
            // Иначе, загружаем форму логина
            loadView('login');
        }

        // Обработчики кликов на навигационные ссылки
        document.querySelectorAll('a[data-view]').forEach(link => {
            link.addEventListener('click', (e) => {
                e.preventDefault();
                const view = e.target.getAttribute('data-view');
                loadView(view);
            });
        });

        // Обработчики IPC-сообщений
        window.api.receive('loginResponse', (data) => {
            if (data.success) {
                userId = data.userId;
                showSuccess('Авторизация успешна!');
                loadView('chat'); // Переход на страницу чата
                initializeWebSocket(); // Инициализация WebSocket
                fetchChats(); // Загрузка списка чатов
            } else {
                showError('Авторизация не удалась.');
            }
        });

        window.api.receive('registerResponse', (data) => {
            if (data.success) {
                showSuccess('Регистрация успешна!');
                loadView('login');
            } else {
                showError('Регистрация не удалась.');
            }
        });

        window.api.receive('logoutResponse', (data) => {
            if (data.success) {
                showSuccess('Вы успешно вышли из системы.');
                // Очищаем локальные данные и закрываем WebSocket
                token = '';
                userId = '';
                if (socket) {
                    socket.disconnect();
                    socket = null;
                }
                loadView('login');
            } else {
                showError('Ошибка при выходе из системы.');
            }
        });

    } catch (error) {
        console.error('Ошибка при инициализации:', error);
        showError('Произошла ошибка при инициализации приложения.');
    }
});

// Функции для отображения уведомлений с помощью SweetAlert2
function showError(message) {
    Swal.fire({
        icon: 'error',
        title: 'Ошибка',
        text: message,
    });
}

function showSuccess(message) {
    Swal.fire({
        icon: 'success',
        title: 'Успех',
        text: message,
    });
}

// Функции для регистрации и логина
async function registerUser(username, password, name, email) {
    try {
        const response = await axios.post(`${API_URL}/api/register`, {
            username,
            password,
            name,
            email
        });

        if (response.status === 200) {
            window.api.send('register', {
                success: true
            });
        }
    } catch (error) {
        console.error('Ошибка регистрации:', error);
        if (error.response && error.response.data && error.response.data.error) {
            window.api.send('register', {
                success: false
            });
            showError(error.response.data.error);
        } else {
            window.api.send('register', {
                success: false
            });
            showError('Ошибка регистрации.');
        }
    }
}

async function loginUser(username, password) {
    try {
        const response = await fetch(`${API_URL}/api/login`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                username: username,
                password: password,
            }),
        });

        const data = await response.json();

        if (data.token) {
            // Успешный логин
            window.api.send('login', {
                token: data.token,
                userId: data.userId
            });
            
            // Немедленный переход в чат
            userId = data.userId;
            showSuccess('Авторизация успешна!');
            loadView('chat');
            initializeWebSocket();
            fetchChats();
        } else {
            showError('Неверные данные для входа.');
        }
    } catch (error) {
        showError('Ошибка при авторизации.');
        console.error('Ошибка:', error);
    }
}

// Функция для загрузки представлений (Login, Registration, Chat)
async function loadView(view) {
    const app = document.getElementById('app');
    const views = {
        login: './templates/Login.html',
        registration: './templates/Registration.html',
        chat: './templates/Chat.html'
    };

    if (!views[view]) return;

    try {
        const response = await fetch(views[view]);
        if (!response.ok) {
            throw new Error('Не удалось загрузить представление.');
        }
        const html = await response.text();
        app.innerHTML = html;

        // Инициализация обработчиков для конкретных представлений
        if (view === 'login') {
            initializeLogin();
        } else if (view === 'registration') {
            initializeRegistration();
        } else if (view === 'chat') {
            initializeChat();
        }
    } catch (error) {
        console.error('Ошибка загрузки представления:', error);
        showError('Ошибка загрузки представления.');
    }
}

// Инициализация формы логина
function initializeLogin() {
    const loginForm = document.getElementById('loginForm');
    if (loginForm) {
        loginForm.addEventListener('submit', function (e) {
            e.preventDefault();

            const username = document.getElementById('username').value.trim();
            const password = document.getElementById('password').value.trim();

            if (username && password) {
                loginUser(username, password);
            } else {
                showError('Пожалуйста, заполните все поля.');
            }
        });
    }

    // Обработка перехода на регистрацию
    const toRegisterLink = document.getElementById('toRegister');
    if (toRegisterLink) {
        toRegisterLink.addEventListener('click', (e) => {
            e.preventDefault();
            loadView('registration');
        });
    }
}

// Инициализация формы регистрации
function initializeRegistration() {
    const registerForm = document.getElementById('registerForm');
    if (registerForm) {
        registerForm.addEventListener('submit', function (e) {
            e.preventDefault();

            const username = document.getElementById('username').value.trim();
            const email = document.getElementById('email').value.trim();
            const password = document.getElementById('password').value.trim();
            const confirmPassword = document.getElementById('confirmPassword').value.trim();

            if (username && email && password && confirmPassword) {
                if (password !== confirmPassword) {
                    showError('Пароли не совпадают.');
                    return;
                }
                if (password.length < 6) {
                    showError('Пароль должен содержать минимум 6 символов.');
                    return;
                }
                registerUser(username, password, username, email); // Предполагаем, что `name` = `username`
            } else {
                showError('Пожалуйста, заполните все поля.');
            }
        });
    }

    // Обработка перехода на логин
    const toLoginLink = document.getElementById('toLogin');
    if (toLoginLink) {
        toLoginLink.addEventListener('click', (e) => {
            e.preventDefault();
            loadView('login');
        });
    }
}

// Инициализация интерфейса чата
function initializeChat() {
    // Добавление кнопки выхода
    const header = document.querySelector('header');
    if (header) {
        const logoutBtn = document.createElement('button');
        logoutBtn.id = 'logoutBtn';
        logoutBtn.className = 'bg-red-500 hover:bg-red-600 text-white px-4 py-2 rounded-md';
        logoutBtn.textContent = 'Выйти';
        header.appendChild(logoutBtn);

        logoutBtn.addEventListener('click', () => {
            window.api.send('logout'); // Отправка сообщения для выхода
        });
    }

    // Загрузка списка чатов
    fetchChats();

    // Обработчик создания нового чата
    const startNewChatBtn = document.getElementById('startNewChatBtn');
    const newChatUsername = document.getElementById('newChatUsername');
    if (startNewChatBtn && newChatUsername) {
        startNewChatBtn.addEventListener('click', () => {
            const username = newChatUsername.value.trim();
            if (username) {
                createNewChat(username);
                newChatUsername.value = ''; // Очистить поле ввода
            } else {
                showError('Пожалуйста, введите логин пользователя');
            }
        });
    }

    // Обработчик выбора чата
    const chatList = document.getElementById('chatList');
    if (chatList) {
        chatList.addEventListener('click', (e) => {
            const chatItem = e.target.closest('.chat-item');
            if (!chatItem) return;

            const chatId = chatItem.dataset.chat;
            currentChat = chatId;
            const chatTitle = document.getElementById('chatTitle');
            const messageInput = document.getElementById('messageInput');
            const sendBtn = document.getElementById('sendBtn');
            
            if (chatTitle) {
                chatTitle.textContent = `Чат ${chatId}`;
            }
            
            // Активируем поле ввода и кнопку отправки
            if (messageInput) messageInput.disabled = false;
            if (sendBtn) sendBtn.disabled = false;
            
            fetchMessages(chatId);
        });
    }

    // Обработчик отправки сообщения
    const sendBtn = document.getElementById('sendBtn');
    if (sendBtn) {
        sendBtn.addEventListener('click', () => {
            const messageInput = document.getElementById('messageInput');
            const message = messageInput.value.trim();
            if (!message || !currentChat) return;

            sendMessage(currentChat, message);
            messageInput.value = '';
        });

        // Обработка нажатия Enter для отправки сообщения
        const messageInput = document.getElementById('messageInput');
        if (messageInput) {
            messageInput.addEventListener('keypress', (e) => {
                if (e.key === 'Enter') {
                    e.preventDefault();
                    sendBtn.click();
                }
            });
        }
    }

    // Обработчик информации о пользователе
    const userInfoBtn = document.getElementById('userInfoBtn');
    if (userInfoBtn) {
        userInfoBtn.addEventListener('click', () => {
            fetchUserProfile(userId);
        });
    }
}

// Функция для создания нового чата с другим пользователем
async function createNewChat(username) {
    try {
        const tokenData = await window.api.getToken();
        const currentToken = tokenData.token;

        if (!currentToken) {
            showError('Токен не найден. Пожалуйста, войдите снова.');
            loadView('login');
            return;
        }

        // Сначала найдем ID пользователя по его username
        const searchResponse = await axios.get(`${API_URL}/api/search?username=${username}`, {
            headers: {
                'Authorization': `Bearer ${currentToken}`
            }
        });

        if (searchResponse.data.users.length === 0) {
            showError('Пользователь не найден');
            return;
        }

        const otherUserId = searchResponse.data.users[0].id;

        const response = await axios.post(`${API_URL}/api/create-chat`, 
            { 
                type: 'personal', // Тип чата: personal или group
                name: `Чат с ${username}`, 
                user_ids: [otherUserId] 
            },
            {
                headers: {
                    'Authorization': `Bearer ${currentToken}`
                }
            }
        );

        if (response.status === 200) {
            const newChat = response.data.chat;
            showSuccess(`Чат с ${username} создан`);
            
            // Обновляем список чатов
            fetchChats();
            
            // Автоматически выбираем новый чат
            currentChat = newChat.id;
            const chatTitle = document.getElementById('chatTitle');
            const messageInput = document.getElementById('messageInput');
            const sendBtn = document.getElementById('sendBtn');
            
            if (chatTitle) {
                chatTitle.textContent = `Чат с ${username}`;
            }
            
            // Активируем поле ввода и кнопку отправки
            if (messageInput) messageInput.disabled = false;
            if (sendBtn) sendBtn.disabled = false;
            
            // Загружаем сообщения (если есть)
            fetchMessages(newChat.id);
        }
    } catch (error) {
        console.error('Ошибка создания чата:', error);
        if (error.response && error.response.data && error.response.data.error) {
            showError(error.response.data.error);
        } else {
            showError('Не удалось создать чат. Возможно, пользователь не найден.');
        }
    }
}

// Инициализация WebSocket-соединения
async function initializeWebSocket() {
    if (socket) {
        return;
    }

    try {
        const tokenData = await window.api.getToken();
        const currentToken = tokenData.token;

        if (!currentToken) {
            showError('Токен не найден. Пожалуйста, войдите снова.');
            loadView('login');
            return;
        }

        socket = io(API_URL, {
            auth: { token: currentToken },
            reconnectionAttempts: 5,  // Set maximum reconnection attempts
            reconnectionDelay: 1000   // Set delay between attempts
        });

        socket.on('connect', () => {
            console.log('WebSocket подключен');
        });

        socket.on('disconnect', () => {
            console.log('WebSocket отключен');
        });

        socket.on('reconnect_failed', () => {
            showError('Не удалось восстановить подключение к серверу.');
        });

        socket.on('message', (data) => {
            const { chat_id, content, sender_id } = data;
            if (currentChat && chat_id.toString() === currentChat.toString()) {
                addMessageToChat(sender_id === userId ? 'me' : 'them', content);
            }
        });

    } catch (error) {
        console.error('Ошибка инициализации WebSocket:', error);
        showError('Не удалось установить соединение с сервером.');
    }
}

// Функция для получения списка чатов
async function fetchChats() {
    try {
        const tokenData = await window.api.getToken();
        const currentToken = tokenData.token;

        console.log('Fetching Chats with Token:', currentToken);

        if (!currentToken) {
            showError('Токен не найден. Пожалуйста, войдите снова.');
            loadView('login');
            return;
        }

        const response = await axios.get(`${API_URL}/api/chats`, {
            headers: {
                'Authorization': `Bearer ${currentToken}`
            }
        });
        if (response.status === 200) {
            renderChatList(response.data.chats);
        } else {
            showError('Ошибка при получении списка чатов.');
        }
    } catch (error) {
        console.error('Ошибка получения чатов:', error);
        if (error.response && error.response.data && error.response.data.error) {
            showError(error.response.data.error);
        } else {
            showError('Ошибка при получении списка чатов.');
        }
    }
}

// Функция для отображения списка чатов
function renderChatList(chats) {
    const chatList = document.getElementById('chatList');
    if (!chatList) return;

    chatList.innerHTML = ''; // Очистка списка

    if (chats.length === 0) {
        chatList.innerHTML = '<p class="text-center text-gray-500 mt-4">Нет доступных чатов</p>';
        return;
    }

    chats.forEach(chat => {
        const chatItem = document.createElement('li');
        chatItem.dataset.chat = chat.id; // Используем ID чата
        chatItem.className = 'chat-item p-4 border-b hover:bg-gray-100 cursor-pointer flex items-center';
        chatItem.innerHTML = `
            <div class="w-10 h-10 bg-blue-200 rounded-full flex items-center justify-center text-blue-600 font-bold">
                ${chat.name.charAt(0).toUpperCase()}
            </div>
            <div class="ml-3">
                <p class="font-medium text-gray-800">${chat.name}</p>
                <p class="text-sm text-gray-500 truncate">Последнее сообщение...</p>
            </div>
        `;
        chatList.appendChild(chatItem);
    });
}

// Функция для получения сообщений чата
async function fetchMessages(chatId) {
    try {
        const tokenData = await window.api.getToken();
        const currentToken = tokenData.token;

        console.log(`Fetching Messages for Chat ID ${chatId} with Token:`, currentToken);

        if (!currentToken) {
            showError('Токен не найден. Пожалуйста, войдите снова.');
            loadView('login');
            return;
        }

        const response = await axios.get(`${API_URL}/api/messages?chat_id=${chatId}`, {
            headers: {
                'Authorization': `Bearer ${currentToken}`
            }
        });
        if (response.status === 200) {
            renderMessages(response.data.messages);
        }
    } catch (error) {
        console.error('Ошибка получения сообщений:', error);
        if (error.response && error.response.data && error.response.data.error) {
            showError(error.response.data.error);
        } else {
            showError('Ошибка при получении сообщений.');
        }
    }
}

// Функция для отображения сообщений
function renderMessages(messages) {
    const chat = document.getElementById('chat');
    if (!chat) return;

    chat.innerHTML = ''; // Очистка чата

    if (messages.length === 0) {
        chat.innerHTML = '<p class="text-gray-500 text-center">Нет сообщений в этом чате</p>';
        return;
    }

    messages.forEach(msg => {
        addMessageToChat(msg.sender_id === userId ? 'me' : 'them', msg.content);
    });

    chat.scrollTop = chat.scrollHeight; // Прокрутка вниз
}

// Функция для отправки сообщения
async function sendMessage(chatId, content) {
    try {
        const tokenData = await window.api.getToken();
        const currentToken = tokenData.token;

        console.log(`Sending Message to Chat ID ${chatId} with Token:`, currentToken);

        if (!currentToken) {
            showError('Токен не найден. Пожалуйста, войдите снова.');
            loadView('login');
            return;
        }

        const response = await axios.post(`${API_URL}/api/message`, {
            chat_id: chatId,
            content: content
        }, {
            headers: {
                'Authorization': `Bearer ${currentToken}`
            }
        });

        if (response.status === 200) {
            // Добавляем сообщение локально
            addMessageToChat('me', content);
            // Отправляем сообщение через WebSocket
            if (socket) {
                socket.emit('message', {
                    chat_id: chatId,
                    content: content
                });
            }
        }
    } catch (error) {
        console.error('Ошибка отправки сообщения:', error);
        if (error.response && error.response.data && error.response.data.error) {
            showError(error.response.data.error);
        } else {
            showError('Ошибка при отправке сообщения.');
        }
    }
}

// Функция для добавления сообщения в чат
function addMessageToChat(sender, content) {
    const chat = document.getElementById('chat');
    if (!chat) return;

    const messageElement = document.createElement('div');
    messageElement.className = `flex items-start space-x-4 ${sender === 'me' ? 'justify-end' : ''}`;

    messageElement.innerHTML = `
        <div class="${sender === 'me' ? 'bg-blue-500 text-white' : 'bg-gray-200 text-gray-800'} p-3 rounded-md max-w-xs">
            <p class="text-sm">${content}</p>
        </div>
    `;
    chat.appendChild(messageElement);
    chat.scrollTop = chat.scrollHeight; // Прокрутка вниз
}

// Функция для получения профиля пользователя
async function fetchUserProfile(userId) {
    try {
        const tokenData = await window.api.getToken();
        const currentToken = tokenData.token;

        console.log(`Fetching Profile for User ID ${userId} with Token:`, currentToken);

        if (!currentToken) {
            showError('Токен не найден. Пожалуйста, войдите снова.');
            loadView('login');
            return;
        }

        const response = await axios.get(`${API_URL}/api/profile/${userId}`, {
            headers: {
                'Authorization': `Bearer ${currentToken}`
            }
        });

        if (response.status === 200) {
            displayUserProfile(response.data.user);
        }
    } catch (error) {
        console.error('Ошибка получения профиля:', error);
        if (error.response && error.response.data && error.response.data.error) {
            showError(error.response.data.error);
        } else {
            showError('Ошибка при получении профиля.');
        }
    }
}

// Функция для отображения профиля пользователя
function displayUserProfile(user) {
    Swal.fire({
        title: 'Профиль пользователя',
        html: `
            <p><strong>Имя:</strong> ${user.name}</p>
            <p><strong>Email:</strong> ${user.email}</p>
            <p><strong>Имя пользователя:</strong> ${user.username}</p>
        `,
        icon: 'info'
    });
}

// Обработчик нажатия кнопки Enter в поле ввода сообщения
document.addEventListener('keypress', (e) => {
    if (e.key === 'Enter') {
        const activeElement = document.activeElement;
        if (activeElement && activeElement.id === 'messageInput') {
            e.preventDefault();
            const sendBtn = document.getElementById('sendBtn');
            if (sendBtn) {
                sendBtn.click();
            }
        }
    }
});

document.getElementById('loginForm').addEventListener('submit', function (e) {
    e.preventDefault();
    const username = document.getElementById('username').value.trim();
    const password = document.getElementById('password').value.trim();

    if (username && password) {
        loginUser(username, password);
    } else {
        showError('Пожалуйста, заполните все поля.');
    }
});

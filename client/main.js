const { app, BrowserWindow, ipcMain } = require('electron');
const path = require('path');

let storedToken = '';  // Объявление переменной на уровне модуля
let storedUserId = '';

function createWindow() {
    const win = new BrowserWindow({
        width: 800,
        height: 600,
        webPreferences: {
            nodeIntegration: false,
            contextIsolation: true,
            preload: path.join(__dirname, 'preload.js') 
        }
    });

    win.loadFile('index.html');
}

app.whenReady().then(createWindow).catch(err => {
    console.error('Ошибка при создании окна:', err);
});

app.on('window-all-closed', () => {
    if (process.platform !== 'darwin') {
        app.quit();
    }
});

app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) {
        createWindow();
    }
});

// Обработчик для получения токена и userId
ipcMain.handle('get-token', () => {
    return { token: storedToken, userId: storedUserId }; // возвращаем сохраненные данные
});

// Обработка IPC-сообщений для логина
ipcMain.on('login', (event, data) => {
    storedToken = data.token;  // Сохраняем данные в глобальных переменных
    storedUserId = data.userId;
});

// Обработчик для выхода
ipcMain.on('logout', (event) => {
    storedToken = '';
    storedUserId = '';
    event.reply('logoutResponse', { success: true });
});

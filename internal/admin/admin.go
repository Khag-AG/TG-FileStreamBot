package admin

import (
    "database/sql"
    "fmt"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/gorilla/websocket"
    _ "github.com/glebarez/sqlite"
)

// Bot структура для хранения информации о боте
type Bot struct {
    ID          string    `json:"id"`
    Token       string    `json:"token"`
    Username    string    `json:"username"`
    ClientName  string    `json:"client_name"`
    ChannelID   string    `json:"channel_id"`
    ChannelName string    `json:"channel_name"`
    Description string    `json:"description"`
    IsActive    bool      `json:"is_active"`
    CreatedAt   time.Time `json:"created_at"`
    LastActive  time.Time `json:"last_active"`
}

// Stats структура для статистики
type Stats struct {
    ActiveBots    int     `json:"active_bots"`
    TotalFiles    int     `json:"total_files"`
    ActiveLinks   int     `json:"active_links"`
    CacheSize     float64 `json:"cache_size_gb"`
    CacheFreeSpace float64 `json:"cache_free_space_percent"`
}

// AdminPanel структура админ панели
type AdminPanel struct {
    db       *sql.DB
    upgrader websocket.Upgrader
}

// NewAdminPanel создает новую админ панель
func NewAdminPanel() (*AdminPanel, error) {
    db, err := sql.Open("sqlite", "./admin.db")
    if err != nil {
        return nil, err
    }

    admin := &AdminPanel{
        db: db,
        upgrader: websocket.Upgrader{
            CheckOrigin: func(r *http.Request) bool {
                return true
            },
        },
    }

    if err := admin.initDB(); err != nil {
        return nil, err
    }

    return admin, nil
}

// initDB инициализирует базу данных
func (a *AdminPanel) initDB() error {
    queries := []string{
        `CREATE TABLE IF NOT EXISTS bots (
            id TEXT PRIMARY KEY,
            token TEXT UNIQUE NOT NULL,
            username TEXT,
            client_name TEXT,
            channel_id TEXT,
            channel_name TEXT,
            description TEXT,
            is_active BOOLEAN DEFAULT true,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            last_active TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`,
        `CREATE TABLE IF NOT EXISTS files (
            id TEXT PRIMARY KEY,
            bot_id TEXT,
            file_id TEXT,
            file_name TEXT,
            file_size INTEGER,
            file_type TEXT,
            download_url TEXT,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            expires_at TIMESTAMP,
            download_count INTEGER DEFAULT 0,
            FOREIGN KEY (bot_id) REFERENCES bots(id)
        )`,
        `CREATE TABLE IF NOT EXISTS settings (
            key TEXT PRIMARY KEY,
            value TEXT
        )`,
    }

    for _, query := range queries {
        if _, err := a.db.Exec(query); err != nil {
            return err
        }
    }

    // Инициализация настроек по умолчанию
    defaultSettings := map[string]string{
        "cache_time_minutes":  "15",
        "max_cache_size_gb":   "10",
        "max_file_size_mb":    "100",
    }

    for key, value := range defaultSettings {
        a.db.Exec("INSERT OR IGNORE INTO settings (key, value) VALUES (?, ?)", key, value)
    }

    return nil
}

// SetupRoutes настраивает маршруты админ панели
    func (a *AdminPanel) SetupRoutes(router *gin.Engine) {
        fmt.Println("DEBUG: Starting SetupRoutes")
        
        admin := router.Group("/admin")
        fmt.Println("DEBUG: Created /admin group")
        
        // HTML страница
        admin.GET("/", a.handleAdminPage)
        fmt.Println("DEBUG: Added GET / route")
        
        admin.GET("", a.handleAdminPage)
        fmt.Println("DEBUG: Added GET '' route")
        
        // Статические файлы
        router.Static("/static", "./internal/admin/static")
        fmt.Println("DEBUG: Added static files route")
        
        // API endpoints
        api := router.Group("/api")
        fmt.Println("DEBUG: Created /api group")
        
        api.GET("/stats", a.handleGetStats)
        fmt.Println("DEBUG: Added /api/stats")
        
        api.GET("/bots", a.handleGetBots)
        fmt.Println("DEBUG: Added /api/bots")
        
        api.POST("/bots", a.handleAddBot)
        fmt.Println("DEBUG: Added POST /api/bots")
        
        api.GET("/files", a.handleGetFiles)
        fmt.Println("DEBUG: Added /api/files")
        
        api.GET("/ws", a.handleWebSocket)
        fmt.Println("DEBUG: Added /api/ws")
        
        fmt.Println("DEBUG: SetupRoutes completed successfully")
    }
}

// handleAdminPage отдает HTML страницу
func (a *AdminPanel) handleAdminPage(c *gin.Context) {
    c.File("./internal/admin/static/index.html")
}

// handleGetStats возвращает статистику
func (a *AdminPanel) handleGetStats(c *gin.Context) {
    var stats Stats
    
    a.db.QueryRow("SELECT COUNT(*) FROM bots WHERE is_active = true").Scan(&stats.ActiveBots)
    a.db.QueryRow("SELECT COUNT(*) FROM files").Scan(&stats.TotalFiles)
    a.db.QueryRow("SELECT COUNT(*) FROM files WHERE expires_at > datetime('now')").Scan(&stats.ActiveLinks)
    
    stats.CacheSize = 2.4
    stats.CacheFreeSpace = 85.0
    
    c.JSON(200, stats)
}

// handleGetBots возвращает список ботов
func (a *AdminPanel) handleGetBots(c *gin.Context) {
    rows, err := a.db.Query(`
        SELECT id, token, username, client_name, channel_id, 
               channel_name, description, is_active, created_at, last_active
        FROM bots
        ORDER BY created_at DESC
    `)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()

    var bots []Bot
    for rows.Next() {
        var bot Bot
        err := rows.Scan(&bot.ID, &bot.Token, &bot.Username, &bot.ClientName,
            &bot.ChannelID, &bot.ChannelName, &bot.Description,
            &bot.IsActive, &bot.CreatedAt, &bot.LastActive)
        if err != nil {
            continue
        }
        bots = append(bots, bot)
    }

    c.JSON(200, bots)
}

// handleAddBot добавляет нового бота
func (a *AdminPanel) handleAddBot(c *gin.Context) {
    var bot Bot
    if err := c.ShouldBindJSON(&bot); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    bot.ID = fmt.Sprintf("%d", time.Now().UnixNano())
    bot.CreatedAt = time.Now()
    bot.LastActive = time.Now()
    bot.IsActive = true

    _, err := a.db.Exec(`
        INSERT INTO bots (id, token, username, client_name, channel_id, 
                         channel_name, description, is_active, created_at, last_active)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `, bot.ID, bot.Token, bot.Username, bot.ClientName, bot.ChannelID,
       bot.ChannelName, bot.Description, bot.IsActive, bot.CreatedAt, bot.LastActive)

    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    c.JSON(200, bot)
}

// handleGetFiles возвращает список файлов
func (a *AdminPanel) handleGetFiles(c *gin.Context) {
    limit := c.DefaultQuery("limit", "50")
    offset := c.DefaultQuery("offset", "0")

    rows, err := a.db.Query(`
        SELECT id, bot_id, file_id, file_name, file_size, 
               file_type, download_url, created_at, expires_at, download_count
        FROM files
        ORDER BY created_at DESC
        LIMIT ? OFFSET ?
    `, limit, offset)
    
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()

    files := []gin.H{}
    c.JSON(200, files)
}

// handleWebSocket обрабатывает WebSocket соединения
func (a *AdminPanel) handleWebSocket(c *gin.Context) {
    conn, err := a.upgrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        return
    }
    defer conn.Close()

    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            var stats Stats
            a.db.QueryRow("SELECT COUNT(*) FROM bots WHERE is_active = true").Scan(&stats.ActiveBots)
            a.db.QueryRow("SELECT COUNT(*) FROM files").Scan(&stats.TotalFiles)
            
            if err := conn.WriteJSON(stats); err != nil {
                return
            }
        }
    }
}

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// Структура настроек Диспетчера
type Config struct {
	ExpressionsEndpoint      string `json:"expressionsEndpoint"`
	UserRegistrationEndpoint string `json:"userRegistrationEndpoint"`
	UserLoginEndpoint        string `json:"userLoginEndpoint"`
	SecretSignJWT            string `json:"secretSignJwt"`
	ExpirationTimeSecondsJWT int    `json:"expirationTimeSecondsJwt"`

	ServerAddress          string `json:"serverAddress"`
	MaxLenghtRequestId     int    `json:"maxLenghtRequestId"`
	UsePrepareValidation   bool   `json:"usePrepareValidation"`
	PrepareValidationRegex string `json:"prepareValidationRegex"`
	MaxLenghtExpression    int    `json:"maxLenghtExpression"`
	UseLocalRequestIdCache bool   `json:"useLocalRequestIdCache"`
	UseLocalResultCache    bool   `json:"useLocalResultCache"`

	DBConnectionConfig struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		DBName   string `json:"dbName"`
	} `json:"dbConnectionConfig"`
}

// Получение настроек из файла настроек
func loadConfig(filename string) (Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return Config{}, err
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return Config{}, err
	}

	return config, nil
}

// Подключение к базе данных
func createDBConnection(config Config) (*sql.DB, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.DBConnectionConfig.Host,
		config.DBConnectionConfig.Port,
		config.DBConnectionConfig.User,
		config.DBConnectionConfig.Password,
		config.DBConnectionConfig.DBName,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	// Проверка подключения к базе данных
	err = db.Ping()
	if err != nil {
		log.Println("Error ping DB connection:", err)
		return nil, err
	}

	return db, nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - -

type User struct {
	ID       int64  `json:"id"`
	Login    string `json:"login"`
	Password string `password:"id"`
}

func getUserByLogin(db *sql.DB, userLogin string) (*User, error) {
	row := db.QueryRow(`SELECT id, login, password FROM users WHERE login = $1`, userLogin)

	var user User
	err := row.Scan(&user.ID, &user.Login, &user.Password)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &user, nil
}

type UserCredentials struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// Хеширование пароля
func getPasswordHash(s string) (string, error) {
	saltedBytes := []byte(s)
	hashedBytes, err := bcrypt.GenerateFromPassword(saltedBytes, bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	hash := string(hashedBytes[:])
	return hash, nil
}

// Сравнения хэша и пароля
func CompareHashAndPassword(hash string, s string) error {
	incoming := []byte(s)
	existing := []byte(hash)
	return bcrypt.CompareHashAndPassword(existing, incoming)
}

func generateJWTToken(userID int64, config Config) (string, error) {

	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userID": userID,
		"nbf":    now.Unix(),                                                                   //время, с которого токен станет валидным
		"exp":    now.Add(time.Duration(config.ExpirationTimeSecondsJWT) * time.Second).Unix(), //время, с которого токен перестанет быть валидным ("протухнет")
		"iat":    now.Unix(),                                                                   //время создания токена
	})

	tokenString, err := token.SignedString([]byte(config.SecretSignJWT))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// Регистрация нового пользователя
func registrationNewUser(db *sql.DB, userLogin, userPassword string) error {

	hashedUserPassword, err := getPasswordHash(userPassword)
	if err != nil {
		return err
	}

	_, err = db.Exec(`INSERT INTO users (login, password) VALUES ($1, $2)`, userLogin, hashedUserPassword)
	if err != nil {
		return err
	}

	return nil
}

// Обработчик регистрации
func userRegistration(w http.ResponseWriter, r *http.Request, db *sql.DB) {

	if r.Method != "POST" {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		log.Printf("Method Not Allowed: %s", r.Method)
		return
	}

	var credentials UserCredentials
	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		log.Println("Error decoding request body:", err)
		return
	}

	// Проверка на отсутствие login или password
	if credentials.Login == "" || credentials.Password == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		log.Println("Incorrect request parameters")
		return
	}

	// Проверка что login или password не пустые
	userLogin := strings.TrimSpace(credentials.Login)
	userPassword := strings.TrimSpace(credentials.Password)
	if userLogin == "" || userPassword == "" {
		http.Error(w, "Login or password is empty", http.StatusBadRequest)
		log.Println("Login or password is empty")
		return
	}

	// Получаем пользователя из БД
	userFromDB, err := getUserByLogin(db, userLogin)
	if err != nil {
		log.Println("Error getting the user from the database: ", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if userFromDB != nil {

		// Пользователь с данным логином уже зарегистрирован
		http.Error(w, "User already exists", http.StatusBadRequest)
		log.Println("User already exists")

	} else {

		// Пользователь с данным логином не найден, регистрируем
		err = registrationNewUser(db, credentials.Login, credentials.Password)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			log.Println("Error registering new user: ", err)
			return
		}

		w.WriteHeader(http.StatusCreated)
		fmt.Fprintln(w, "User registered successfully")
		log.Println("User registered successfully")

	}
}

// Обработчик аутентификации
func userLogin(w http.ResponseWriter, r *http.Request, config Config, db *sql.DB) {
	if r.Method != "POST" {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		log.Printf("Method Not Allowed: %s", r.Method)
		return
	}

	var credentials UserCredentials
	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		log.Println("Error decoding request body:", err)
		return
	}

	// Проверка на отсутствие login или password
	if credentials.Login == "" || credentials.Password == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		log.Println("Incorrect request parameters")
		return
	}

	// Проверка что login или password не пустые
	userLogin := strings.TrimSpace(credentials.Login)
	userPassword := strings.TrimSpace(credentials.Password)
	if userLogin == "" || userPassword == "" {
		http.Error(w, "Login or password is empty", http.StatusBadRequest)
		log.Println("Login or password is empty")
		return
	}

	// Получаем пользователя из БД
	userFromDB, err := getUserByLogin(db, userLogin)
	if err != nil {
		log.Println("Error getting the user from the database: ", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if userFromDB == nil {

		// Пользователь с данным логином не найден, возвращаем ошибку
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		log.Println("User not found")

	} else {

		// Пользователь с данным логином уже зарегистрирован, проверим переданный пароль
		err := CompareHashAndPassword(userFromDB.Password, userPassword)
		if err != nil {
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
			log.Println("Incorrect password: ", err)
			return
		}

		// Пароль верный, пользователь аутентифицирован
		// Генерируем JWT токен
		token, err := generateJWTToken(userFromDB.ID, config)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			log.Println("Error generating JWT token: ", err)
			return
		}

		// Отправляем токен в теле ответа
		response := map[string]string{"token": token}
		jsonResponse, err := json.Marshal(response)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			log.Println("Error marshalling JSON response:", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonResponse)
		// log.Println(w, "User authentication was successful")
		log.Printf("User %s authentication was successful", userFromDB.Login)

	}
}

// Проверка и валидация JWT токена во входящем запросе
func validateJWTToken(r *http.Request, config Config) (*jwt.Token, error) {

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("missing JWT token")
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		return nil, fmt.Errorf("invalid JWT token format")
	}

	// Валидация токена
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Проверяем что используется правильный алгоритм подписи
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		// Возвращаем секретный ключ для проверки подписи
		return []byte(config.SecretSignJWT), nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %v", err)
	}

	// Проверяем валидность и тип токена
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return token, nil
}

// Получение идентификатора пользователя из токена
func getUserIDFromJWTToken(token *jwt.Token) (int, error) {

	userID, ok := token.Claims.(jwt.MapClaims)["userID"].(float64)
	if !ok {
		return 0, fmt.Errorf("invalid user ID")
	}

	return int(userID), nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
var requestIdCache sync.Map // Кэш RequestID
var resultCache sync.Map    // Кэш готовых результатов

func getRequestIDFromHeader(r *http.Request, config Config) (string, error) {

	requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))

	if requestID == "" {
		return "", fmt.Errorf("X-Request-ID is missing or empty")
	}

	if config.MaxLenghtRequestId > 0 && (len(requestID) > config.MaxLenghtRequestId) {
		return "", fmt.Errorf("X-Request-ID exceeds the maximum length of %d characters", config.MaxLenghtRequestId)
	}

	return requestID, nil
}

func searchRequestInDB(db *sql.DB, requestID string) (string, error) {
	var taskID string
	err := db.QueryRow("SELECT id FROM reg_expr WHERE request_id = $1 ORDER BY reg_date DESC LIMIT 1", requestID).Scan(&taskID) //берем последнюю(т.е. недавнюю) запись
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return taskID, nil
}

func searchTaskIdByRequestId(requestID string, config Config, db *sql.DB) (string, error) {

	// Ищем в локальном кэше
	if config.UseLocalRequestIdCache {
		if cachedTaskID, ok := requestIdCache.Load(requestID); ok {
			if cachedTaskID != "" {
				// Нашли
				return cachedTaskID.(string), nil
			} else {
				requestIdCache.Delete(requestID)
			}
		}
	}

	// В кэше не нашли, ищем в БД
	taskID, err := searchRequestInDB(db, requestID)
	if err != nil {
		return "", fmt.Errorf("error getting result from database(searchRequestInDB): %v", err)
	}

	// Если нашли, добавляем в локальный кэш
	if config.UseLocalRequestIdCache && taskID != "" {
		if _, ok := requestIdCache.Load(requestID); ok {
		} else {
			requestIdCache.Store(requestID, taskID)
		}
	}

	return taskID, nil
}

func returnTaskIdToClient(w http.ResponseWriter, taskID string) error {
	response := map[string]string{"task_id": taskID}
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("error encoding JSON")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseJSON)

	return nil
}

func returnTaskInfoToClient(w http.ResponseWriter, TaskInfo *TaskInfoStruct) error {
	responseJSON, err := json.Marshal(TaskInfo)
	if err != nil {
		return fmt.Errorf("error encoding JSON")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseJSON)

	return nil
}

// Структура входящего запроса(тела) с выражением для вычисления
type ExpressionMessage struct {
	Expression string `json:"expression"`
}

func isValidArithmeticExpression(expr string, config Config) bool {
	validChars := regexp.MustCompile(config.PrepareValidationRegex)
	if !validChars.MatchString(expr) {
		return false
	}

	// Проверка корректности скобок
	bracketBalance := 0
	for _, char := range expr {
		if char == '(' {
			bracketBalance++
		} else if char == ')' {
			bracketBalance--
			if bracketBalance < 0 {
				return false
			}
		}
	}

	return bracketBalance == 0
}

func getCurrExpression(r *http.Request, config Config) (string, error) {
	var exprMessage ExpressionMessage

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&exprMessage); err != nil {
		return "", err
	}

	CurrExpression := strings.TrimSpace(exprMessage.Expression)

	// Проверка корректности полученного выражения
	if CurrExpression == "" {
		return "", fmt.Errorf("invalid arithmetic expression")
	}

	if config.MaxLenghtExpression > 0 && (len(CurrExpression) > config.MaxLenghtExpression) {
		return "", fmt.Errorf("expression exceeds the maximum length of %d characters", config.MaxLenghtExpression)
	}

	// Предварительная валидация выражения
	if config.UsePrepareValidation {
		if !isValidArithmeticExpression(CurrExpression, config) {
			return "", fmt.Errorf("invalid arithmetic expression")
		}
	}

	return CurrExpression, nil
}

// Записываем(регистрируем) задачу непосредственно в базу данных
func insertExpressionToDB(db *sql.DB, userID int, requestID string, CurrExpression string) (string, error) {
	var taskID string
	err := db.QueryRow("INSERT INTO reg_expr(user_id, request_id, reg_date, expression) VALUES($1, $2, $3, $4) RETURNING id", userID, requestID, time.Now(), CurrExpression).Scan(&taskID)
	if err != nil {
		return "", err
	}
	return taskID, nil
}

// Записываем(регистрируем) задачу в хранилище
func pushToStorage(config Config, userID int, requestID string, CurrExpression string, db *sql.DB) (string, error) {

	taskID, err := insertExpressionToDB(db, userID, requestID, CurrExpression)
	if err != nil {
		return "", fmt.Errorf("error inserting expression into database(insertExpressionToDB): %v", err)
	}

	// Добавляем в локальный кэш
	if config.UseLocalRequestIdCache && taskID != "" {
		if _, ok := requestIdCache.Load(requestID); ok {
		} else {
			requestIdCache.Store(requestID, taskID)
		}
	}

	return taskID, nil
}

// Структура атрибутов задачи для возврата клиенту
type TaskInfoStruct struct {
	Expression   string `json:"expression"`
	Result       string `json:"result"`
	Status       string `json:"status"`
	IsFinished   bool   `json:"is_finished"`
	IsWrong      bool   `json:"is_wrong"`
	CalcDuration int64  `json:"calc_duration"`
	Comment      string `json:"comment"`
}

// Получаем идентификатор задачи из GET параметров
func getTaskIDFromParams(r *http.Request) (int, error) {

	taskID := strings.TrimSpace(r.URL.Query().Get("task_id"))

	if taskID == "" {
		return 0, fmt.Errorf("task_id is missing or empty")
	}

	// Попытка преобразовать taskID в число
	taskIDInt, err := strconv.Atoi(taskID)
	if err != nil {
		return 0, fmt.Errorf("task_id is missing")
	}

	// Проверка, что строковое представление преобразованного числа совпадает с исходной строкой
	if strconv.Itoa(taskIDInt) != taskID {
		return 0, fmt.Errorf("task_id is missing")
	}

	return taskIDInt, nil
}

func getResultInfoFromDatabase(db *sql.DB, taskID int, userID int) (*TaskInfoStruct, error) {
	var TaskInfo TaskInfoStruct
	err := db.QueryRow(`
		SELECT 
			expression, 
			result, 
			COALESCE(status, '') AS status,
			is_finished,
			is_wrong,
			calc_duration,
			COALESCE(comment, '') AS comment
		FROM reg_expr 
		WHERE id = $1 AND user_id = $2
	`, taskID, userID).Scan(
		&TaskInfo.Expression,
		&TaskInfo.Result,
		&TaskInfo.Status,
		&TaskInfo.IsFinished,
		&TaskInfo.IsWrong,
		&TaskInfo.CalcDuration,
		&TaskInfo.Comment,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("record not found for task_id: %d user_id: %d", taskID, userID)
		}
		return nil, fmt.Errorf("error getting result from database: %v", err)
	}
	return &TaskInfo, nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func addingNewExpressionToCalc(w http.ResponseWriter, r *http.Request, config Config, db *sql.DB) {

	// Валидируем JWT токен
	token, err := validateJWTToken(r, config)
	if err != nil {
		http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
		log.Println("Unauthorized:", err)
		return
	}

	// Токен валиден, получим идентификатор пользователя
	userID, err := getUserIDFromJWTToken(token)
	if err != nil {
		http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
		log.Println("Unauthorized:", err)
		return
	}

	// Получаем идентификатор запроса
	requestID, err := getRequestIDFromHeader(r, config)
	if err != nil {
		http.Error(w, "Invalid requestID: "+err.Error(), http.StatusBadRequest)
		log.Println("Invalid requestID:", err)
		return
	}

	// Ищем таску в кэше и БД (обеспечиваем идемпотентность)
	taskID, err := searchTaskIdByRequestId(requestID, config, db)
	if err != nil {
		http.Error(w, "Internal Server Error: "+err.Error(), http.StatusInternalServerError)
		log.Println("Error searching taskID by requestID:", err)
		return
	}

	if taskID != "" {
		// Нашли таску, возвращаем клиенту taskID(обеспечиваем идемпотентность)
		err := returnTaskIdToClient(w, taskID)
		if err != nil {
			http.Error(w, "Internal Server Error: "+err.Error(), http.StatusInternalServerError)
			log.Println("Error returning the task_id:", err)
		}
		return
	}

	// Не нашли taskID по requestID, поэтому считаем, что это новый запрос на вычисление

	// Извлекаем выражение
	CurrExpression, err := getCurrExpression(r, config)
	if err != nil {
		http.Error(w, "Internal Server Error: "+err.Error(), http.StatusBadRequest)
		log.Println("Expression extraction error: ", err)
		return
	}

	// Сохраняем выражение в базе данных
	taskID, err = pushToStorage(config, userID, requestID, CurrExpression, db)
	if err != nil {
		http.Error(w, "Internal Server Error: "+err.Error(), http.StatusInternalServerError)
		log.Println("Internal Server Error: ", err)
		return
	}

	returnTaskIdToClient(w, taskID)
}

func getCalcResult(w http.ResponseWriter, r *http.Request, config Config, db *sql.DB) {

	// Валидируем JWT токен
	token, err := validateJWTToken(r, config)
	if err != nil {
		http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
		log.Println("Unauthorized:", err)
		return
	}

	// Токен валиден, получим идентификатор пользователя
	userID, err := getUserIDFromJWTToken(token)
	if err != nil {
		http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
		log.Println("Unauthorized:", err)
		return
	}

	// Получаем идентификатор задачи
	taskID, err := getTaskIDFromParams(r)
	if err != nil {
		http.Error(w, "Invalid task_id: "+err.Error(), http.StatusBadRequest)
		log.Println("Invalid task_id:", err)
		return
	}

	// Ищем информацию по задаче в кэше готовых результатов(где IsFinished=true)
	if config.UseLocalResultCache {
		if cachedTaskInfo, ok := resultCache.Load(taskID); ok {
			// Нашли - возвращаем клиенту
			err := returnTaskInfoToClient(w, cachedTaskInfo.(*TaskInfoStruct))
			if err != nil {
				http.Error(w, "Internal Server Error: "+err.Error(), http.StatusInternalServerError)
				log.Println("Error returning the task_id:", err)
			}
			return
		}
	}

	// В кэше не нашли, получим из БД
	TaskInfo, err := getResultInfoFromDatabase(db, taskID, userID)
	if err != nil {
		// Проверяем тип ошибки
		if strings.HasPrefix(err.Error(), "record not found for task_id") {
			// Ошибка клиента
			http.Error(w, fmt.Sprintf("Record not found for task_id: %d", taskID), http.StatusBadRequest)
		} else {
			// Ошибка сервера
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		log.Printf("Error getting result from database(getResultFromDatabase): %v", err)
		return
	}

	// Добавим в локальный кэш
	if config.UseLocalResultCache && TaskInfo.IsFinished {
		if _, ok := resultCache.Load(taskID); ok {
		} else {
			resultCache.Store(taskID, TaskInfo)
		}
	}

	err = returnTaskInfoToClient(w, TaskInfo)
	if err != nil {
		http.Error(w, "Internal Server Error: "+err.Error(), http.StatusInternalServerError)
		log.Println("Error returning the TaskInfo:", err)
	}
}

func processingCalcRequest(w http.ResponseWriter, r *http.Request, config Config, db *sql.DB) {
	switch r.Method {
	case "POST":
		addingNewExpressionToCalc(w, r, config, db)
	case "GET":
		getCalcResult(w, r, config, db)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		log.Printf("Method Not Allowed: %s", r.Method)
		return
	}
}

//- - - - - - - - - - - - - - - - - - - - - - - - - - - - -

// Определение веб-хендлеров
func defineWebHandlers(config Config, db *sql.DB) {

	// Определение обработчика запросов регистрации пользователей
	http.HandleFunc(config.UserRegistrationEndpoint, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID")

		if r.Method == "OPTIONS" {
			return
		}

		userRegistration(w, r, db)
	})

	// Определение обработчика запросов аутентификации пользователей
	http.HandleFunc(config.UserLoginEndpoint, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID")

		if r.Method == "OPTIONS" {
			return
		}

		userLogin(w, r, config, db)
	})

	// Определение обработчика запросов с выражениями
	http.HandleFunc(config.ExpressionsEndpoint, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID, Authorization")

		if r.Method == "OPTIONS" {
			return
		}

		processingCalcRequest(w, r, config, db)
	})
}

//- - - - - - - - - - - - - - - - - - - - - - - - - - - - -

func main() {

	// Загружаем настройки Диспетчера
	config, err := loadConfig("dispatcher_config.json")
	if err != nil {
		log.Println("Error loading Dispatcher config:", err)
		return
	}

	// Создание подключения к базе данных
	db, err := createDBConnection(config)
	if err != nil {
		log.Println("Error creating DB connection:", err)
		return
	}
	defer db.Close()

	// Определение обработчиков веб-запросов
	defineWebHandlers(config, db)

	// Запуск HTTP сервера
	go func() {
		log.Println("Server 'Dispatcher' is listening on", config.ServerAddress)
		log.Fatal(http.ListenAndServe(config.ServerAddress, nil))
	}()

	// Бесконечный цикл(для работы горутин)
	select {}
}

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pb "ya_lms_expression_calc_two/grpc/proto_exchange"
	"ya_lms_expression_calc_two/internal/expression_processing"
)

var (
	grpcSrvAddress    string               // Адрес и порт данного экземпляра Демона
	startTime         time.Time            // Время запуска данного экземпляра Демона
	grpcClient        pb.CalcManagerClient // Клиент для обмена с gRPC-сервисом Менеджера
	grpcClientMutex   sync.Mutex           // Для защиты доступа к grpcClient
	calcQueue         sync.Map             // Очередь Демона(ключ:RecordID, значение:Параметры задачи)
	taskStatus        sync.Map             // Карта состояний задач(ключ:RecordID, значение:состояние задачи{"queued", "running", "completed"})
	qntProcessedTasks int                  // Счетчик задач, побывавших в очереди Демона(обработанных)

)

//-----------------------------

const (
	queued    = "queued"    // Задача в свободна
	running   = "running"   // Задача обрабатывается
	completed = "completed" // Задача завершена
)

// Установка у задачи статуса "queued"(т.е. свободна)
func initTaskStatus(RecordID int64) {
	taskStatus.Store(RecordID, queued)
}

// Установка у задачи статуса "running"(т.е. в обработке)
func setTaskRunning(RecordID int64) {
	taskStatus.Store(RecordID, running)
}

// Установка у задачи статуса "completed"(т.е. завершена)
func setTaskCompleted(RecordID int64) {
	taskStatus.Store(RecordID, completed)
}

// Проверка, что задача в очереди Демона и у неё статус "queued"(т.е. свободна)
func isTaskQueued(RecordID int64) bool {
	status, ok := taskStatus.Load(RecordID)
	if !ok {
		return false
	}
	return status == queued
}

//-------------------------------

// Структура настроек Демона
type Config struct {
	ServerAddress                      string `json:"serverAddress"` // Адрес текущего экземпляра Демона(без порта)
	ServerPort                         string `json:"defaultPort"`   // Порт на котором запущен текущий экземпляр Демона
	ConfigEndpoint                     string `json:"configEndpoint"`
	ManagerGRPCServer                  string `json:"managerGRPCServer"`                  // Адрес и порт gRPC-сервера Менеджера
	ManagerGRPCServerReconnectInterval int    `json:"managerGRPCServerReconnectInterval"` // Интервал реконнектов к gRPC-серверу Менеджера(в сек.)
	EchoRepeatInterval                 int    `json:"echoRepeatInterval"`                 // Интервал Echo-запросов к gRPC-серверу Менеджера(в сек.)
	MaxLenghtCalcQueue                 int    `json:"maxLenghtCalcQueue"`                 // Максимальное значение размера очереди заданий Демона
	DelayBasicLoop                     int    `json:"delayBasicLoop"`                     // Задержка итерации главного цикла(в сек.)
	DelayAddingLoop                    int    `json:"delayAddingLoop"`                    // Задержка итерации дополнительного цикла(в сек.)
	MaxGoroutinesInCalcProcess         int    `json:"maxGoroutinesInCalcProcess"`         // Максимальное количество "параллельных" горутин для вычисления одного выражения
	AdditionalDelayInCalcOperation     int    `json:"additionalDelayInCalcOperation"`     // Дополнительная задержка каждой вычислительной подоперации(в сек.)
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

//-----------------------------

// connectToGRPCServer устанавливает соединение с gRPC-сервером
func connectToGRPCServer(config Config) pb.CalcManagerClient {

	for {
		conn, err := grpc.NewClient(config.ManagerGRPCServer, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("Could not connect to Manager gRPC-server: %v. Retrying in %d seconds...", err, config.ManagerGRPCServerReconnectInterval)
			time.Sleep(time.Duration(config.ManagerGRPCServerReconnectInterval) * time.Second)
			continue
		}
		return pb.NewCalcManagerClient(conn)
	}
}

// Цикл выполнения Echo-запросов к (gRPC-сервису) Менеджера
func runEchoLoop(config *Config) {

	for {
		echoProcessing(config)

		time.Sleep(time.Duration(config.EchoRepeatInterval) * time.Second)
	}
}

func echoProcessing(config *Config) {

	err := sendEchoRequest(config)
	if err != nil {
		if status.Code(err) == codes.Unavailable {
			fmt.Printf("Manager gRPC-server is down. Trying to reconnect in %d seconds …\n", config.EchoRepeatInterval)
		} else {
			log.Printf("Error calling remote procedure(gRPC Echo-request): %v", err)
		}
	}
}

// Отправка Echo-запроса Менеджеру(он же Health-check)
func sendEchoRequest(config *Config) error {

	grpcClientMutex.Lock()
	_, err := grpcClient.Echo(context.TODO(), &pb.EchoRequest{
		Address:   fmt.Sprintf("%s:%s", config.ServerAddress, config.ServerPort),
		StartTime: startTime.Format(time.RFC3339),
		Capacity:  int32(config.MaxLenghtCalcQueue),
		Queue:     int32(calcQueueSize()),
	})
	grpcClientMutex.Unlock()

	return err
}

// Отправка Result-запроса(т.е. с результатами вычисления) Менеджеру
func sendResultRequest(task *Task) (string, error) {

	log.Printf("Sending gRPC Result-request of task with RecordID: %d ...", task.RecordID)

	grpcClientMutex.Lock()
	res, err := grpcClient.Result(context.TODO(), &pb.ResultRequest{
		Address:       grpcSrvAddress,
		RecordId:      task.RecordID,
		TaskId:        task.TaskID,
		IsWrong:       task.IsWrong,
		IsFinished:    task.IsFinished,
		Comment:       task.Comment,
		RpnExpression: task.RPNexpression,
		Result:        task.Result,
	})
	grpcClientMutex.Unlock()

	if err != nil {
		return "", err
	}

	return res.Status, nil
}

//----------

type Server struct {
	config Config
	pb.CalcDaemonServer
}

func NewServer(config Config) *Server {
	return &Server{
		config: config,
	}
}

func (s *Server) AddTask(ctx context.Context, in *pb.AddTaskRequest) (*pb.AddTaskResponse, error) {
	log.Println("Incoming AddTask request with params: ", in)
	task := Task{
		RecordID:      in.RecordId,
		TaskID:        in.TaskId,
		Expression:    in.Expression,
		RPNexpression: in.RpnExpression,
	}

	// Проверим, м.б. задача уже в очереди
	_, found := getTaskFromCache(task.RecordID)
	if found {
		// Задача уже в очереди, не добавляем
		return &pb.AddTaskResponse{Status: "OK"}, nil
	}

	// Проверим наличие свободных мест в очереди
	if s.config.MaxLenghtCalcQueue > calcQueueSize() {
		// Добавляем в очередь
		addTaskToCache(task.RecordID, &task)
		return &pb.AddTaskResponse{Status: "OK"}, nil
	} else {
		// Свободных мест нет
		log.Println("No empty spaces in the queue!")
		return &pb.AddTaskResponse{Status: "FULL"}, nil
	}
}

type Task struct {
	RecordID      int64   `json:"record_id"`
	TaskID        int64   `json:"task_id"`
	Expression    string  `json:"expression"`
	RPNexpression string  `json:"rpn_expression"`
	IsWrong       bool    `json:"is_wrong"`
	IsFinished    bool    `json:"is_finished"`
	Comment       string  `json:"comment"`
	Result        float64 `json:"result"`
}

// Добавление задачи в очередь Демона
func addTaskToCache(RecordID int64, task *Task) {

	task.IsWrong = false
	task.IsFinished = false
	calcQueue.Store(RecordID, task)

	initTaskStatus(RecordID) // установка начального состояния в карте состояния задачи
	qntProcessedTasks++
}

// Возвращает длину calcQueueSize
func calcQueueSize() int {
	var size int
	calcQueue.Range(func(_, _ interface{}) bool {
		size++
		return true
	})
	return size
}

// Получение задачи из очереди Демона
func getTaskFromCache(RecordID int64) (*Task, bool) {

	cachedTask, ok := calcQueue.Load(RecordID)
	if !ok {
		return nil, false
	}

	task, ok := cachedTask.(*Task)
	if !ok {
		return nil, false
	}

	return task, true
}

//-----------------------------

// Проверка допустимости использования оператора
func isOperator(token string) bool {
	if token == "+" || token == "-" || token == "*" || token == "/" || token == "^" {
		return true
	}
	return false
}

type RPNTuple struct {
	TokenA       string
	TokenB       string
	Operation    string
	Result       float64
	IsWrong      bool
	ErrorComment string
}

// Применение операции к RPN-подвыражению
func executeOperation(config *Config, tokenA, tokenB, operation string) (float64, error) {

	// Доп.задержка
	time.Sleep(time.Duration(config.AdditionalDelayInCalcOperation) * time.Second)

	operandA, errA := strconv.ParseFloat(tokenA, 64)
	operandB, errB := strconv.ParseFloat(tokenB, 64)
	if errA != nil || errB != nil {
		return 0, errors.New("error parsing operands")
	}

	switch operation {
	case "+":
		return operandA + operandB, nil
	case "-":
		return operandA - operandB, nil
	case "*":
		return operandA * operandB, nil
	case "/":
		if operandB == 0 {
			return 0, errors.New("division by zero")
		}
		return operandA / operandB, nil
	case "^":
		return math.Pow(operandA, operandB), nil
	default:
		return 0, errors.New("unknown operation")
	}
}

// Вычисление RPN-подвыражения
func execSubExpression(subExpr map[string]RPNTuple, config *Config) {
	var wg sync.WaitGroup
	var mutex sync.Mutex
	sem := make(chan struct{}, config.MaxGoroutinesInCalcProcess)

	for key, value := range subExpr {
		sem <- struct{}{} // Захватываем слот в канале (ожидаем, если слоты закончились)

		wg.Add(1)
		go func(key string, value RPNTuple) {
			defer func() {
				<-sem // Освобождаем слот в канале после завершения горутины
				wg.Done()
			}()

			result, err := executeOperation(config, value.TokenA, value.TokenB, value.Operation)
			if err != nil {
				value.IsWrong = true
				value.ErrorComment = err.Error()
			} else {
				value.Result = result
			}

			mutex.Lock()
			subExpr[key] = value
			mutex.Unlock()
		}(key, value)

	}
	wg.Wait()
	close(sem) // Закрываем канал после завершения всех горутин
}

// Вычисление RPN-выражения
func evaluateRPN(tokens []string, task *Task, config *Config) (float64, error) {

	var currSlice []string
	currSubExpr := make(map[string]RPNTuple)
	n := 0

	for _, token := range tokens {
		lenSlice := len(currSlice)

		if lenSlice >= 2 {
			if isOperator(token) && !isOperator(currSlice[lenSlice-1]) && !isOperator(currSlice[lenSlice-2]) {
				currTuple := RPNTuple{currSlice[lenSlice-2], currSlice[lenSlice-1], token, 0, false, ""}

				// Формируем имя нового идентификатора для вставки результата выражения
				idForPaste := "op_" + strconv.Itoa(n)
				currSubExpr[idForPaste] = currTuple
				currSlice[lenSlice-2] = idForPaste
				n++
				currSlice = append(currSlice, token)
				continue
			}

			currSlice = append(currSlice, token)

		} else {
			// если тут есть операторы, то это ошибка
			currSlice = append(currSlice, token)
		}
	}

	if len(currSubExpr) > 0 {
		execSubExpression(currSubExpr, config)

		for key, value := range currSubExpr {
			if value.IsWrong {
				// Дальнейший расчет можно не производить
				return 0, errors.New("error in calculating the operation")
			}

			// Находим индекс элемента в currSlice по ключу
			indexToUpdate := -1
			for i, token := range currSlice {
				if token == key {
					indexToUpdate = i
					break
				}
			}

			if indexToUpdate == -1 {
				// Дальнейший расчет можно не производить
				return 0, errors.New("error in calculating the operation")
			} else {
				// Заменяем текущий элемент в currSlice на value.Result, преобразованный в строку
				currSlice[indexToUpdate] = strconv.FormatFloat(value.Result, 'f', -1, 64)

				// Удаляем следующие справа 2 элемента, если они есть
				if len(currSlice) > indexToUpdate+2 {
					currSlice = append(currSlice[:indexToUpdate+1], currSlice[indexToUpdate+3:]...)
				}
			}
		}
	} else {
		if len(currSlice) > 1 {
			// Тройки закончены, но тоены ещё нет - это ошибка в записи выражения
			return 0, errors.New("error parsing final result")
		}
	}

	if len(currSlice) == 1 {
		result, err := strconv.ParseFloat(currSlice[0], 64)
		if err != nil {
			return 0, errors.New("error parsing final result")
		}
		return result, nil
	}

	if len(currSlice) > 0 {
		// Отправка промежуточного результата Менеджеру
		sendIntermediateResultToManager(currSlice, task, config)
		return evaluateRPN(currSlice, task, config)
	}

	return 0, nil
}

// Отправка промежуточного результата Менеджеру
func sendIntermediateResultToManager(currSlice []string, task *Task, config *Config) {

	log.Printf("Intermediate expression: %s", currSlice)

	copiedTask := &Task{
		RecordID:      task.RecordID,
		TaskID:        task.TaskID,
		Expression:    task.Expression,
		RPNexpression: strings.Join(currSlice, " "),
		// RegDateTime:   task.RegDateTime,
		IsWrong:    task.IsWrong,
		IsFinished: task.IsFinished,
		Comment:    task.Comment,
		Result:     task.Result,
	}

	status, err := sendResultRequest(copiedTask)
	if err != nil {
		log.Printf("Error sending intermediate-result of evaluation %d: %v", copiedTask.RecordID, err)
		return
	}

	switch status {
	case "OK":
		//Успешно передали Менеджеру
		log.Printf("gRPC intermediate-request of task with RecordID: %d - was successfully sent to Manager", copiedTask.RecordID)
	default:
		log.Printf("Unknown status received gRPC intermediate-request from Manager. Демон: %s  Status: %s", fmt.Sprintf("%s:%s", config.ServerAddress, config.ServerPort), status)
	}
}

// Вычисление выражения из задачи
func calculateExpressionFromTask(task *Task, config *Config) {

	result, err := evaluateRPN(strings.Fields(task.RPNexpression), task, config)
	if err != nil {
		task.Comment = fmt.Sprintf("Error: %v\n", err)
		task.IsWrong = true
		task.IsFinished = true
	} else {
		task.Result = result
		task.IsWrong = false
		task.IsFinished = true
	}
}

// Запуск обработки задачи
func runTask(RecordID int64, task *Task, config *Config) {

	defer setTaskCompleted(RecordID)

	// Проверяем наличие выражения
	currExpression := strings.TrimSpace(task.Expression)
	if currExpression == "" {
		log.Printf("Error expression for RecordID: %d", RecordID)
		task.Comment = fmt.Sprintf("Error expression for RecordID: %d", RecordID)
		task.IsWrong = true
		task.IsFinished = true

		return
	}

	// Проверяем, передано ли RPN(если нет - вычисляем)
	currRPNExpression := strings.TrimSpace(task.RPNexpression)
	if currRPNExpression == "" {
		postfix, err := expression_processing.Parse(task.Expression)
		if err != nil {
			log.Printf("Error getting RPN for RecordID %d: %v", RecordID, err)
			task.Comment = fmt.Sprintf("Error getting RPN for RecordID: %d", RecordID)
			task.IsWrong = true
			task.IsFinished = true

			return
		}

		// Поскольку, в общем случае, у нас вычислитель может быть на другой машине, то
		// передаем RPN именно как строку(хотя конечно уже есть срез postfix, и мы его ещё раз вычислим)
		task.RPNexpression = expression_processing.PostfixToString(postfix)
	}

	// Вычисление выражения из задачи
	calculateExpressionFromTask(task, config)
}

// Процедура основного цикла
func primaryProcessing(config *Config) {

	calcQueue.Range(func(key, value interface{}) bool {
		RecordID := key.(int64)
		task := value.(*Task)

		// Проверяем, нужно ли обработать задачу
		if isTaskQueued(RecordID) {

			setTaskRunning(RecordID)

			go runTask(RecordID, task, config)
		}

		return true
	})
}

// Основной цикл(вычисление выражений, отправка промежуточных результатов Менеджеру)
func runBasicDaemonLoop(config *Config) {

	for {
		primaryProcessing(config)

		time.Sleep(time.Duration(config.DelayBasicLoop) * time.Second)

	}
}

// Дополнительный цикл(чистка очереди выполненных, отправка финальных результатов Менеджеру)
func runAddingDaemonLoop(config *Config) {

	for {
		secondaryProcessing()

		time.Sleep(time.Duration(config.DelayAddingLoop) * time.Second)
	}
}

func secondaryProcessing() {

	calcQueue.Range(func(key, value interface{}) bool {
		RecordID := key.(int64)
		task := value.(*Task)

		// Обрабатываем завершенные задачи
		if task.IsFinished {
			status, err := sendResultRequest(task)
			if err != nil {
				log.Printf("Error sending result of evaluation %d: %v", RecordID, err)
				return true // Переход к следующей итерации
			}

			switch status {
			case "OK":
				//Успешно передали Менеджеру, чистим очередь
				calcQueue.Delete(RecordID)
				taskStatus.Delete(RecordID)
				log.Printf("gRPC Result-request of task with RecordID: %d - was successfully sent to Manager", RecordID)
			default:
				log.Printf("Unknown status received gRPC Result-request from Manager. Status: %s", status)
			}
		}

		return true

	})
}

//-----------------------------

// DTO для возврата настроек Демона
type ConfigInfo struct {
	StartTime         string     `json:"start_time"` // Время старта Демона
	Uptime            int        `json:"uptime"`     // Аптайм Демона(в сек.)
	QntCurrentTasks   int        `json:"qnt_current_tasks"`
	QntProcessedTasks int        `json:"qntProcessedTasks"`
	CurrentTasks      []TaskInfo `json:"current_tasks"`
	CurrentSettings   Config     `json:"current_settings"`
}

// DTO для возврата списка задач в настройках Демона
type TaskInfo struct {
	RecordID      int64  `json:"record_id"`
	TaskID        int64  `json:"task_id"`
	Expression    string `json:"expression"`
	RPNExpression string `json:"rpn_expression"`
	IsFinished    bool   `json:"is_finished"`
	IsWrong       bool   `json:"is_wrong"`
	Result        string `json:"result"`
	Status        string `json:"status"`
}

// DTO запроса обновления настроек
type SettingsUpdateRequest struct {
	Settings string `json:"settings"`
}

// DTO структура настроек
type SettingsStruct struct {
	// ManagerGRPCServer                  string `json:"managerGRPCServer"`
	// ManagerGRPCServerReconnectInterval int    `json:"managerGRPCServerReconnectInterval"`
	EchoRepeatInterval             int `json:"echoRepeatInterval"`
	MaxLenghtCalcQueue             int `json:"maxLenghtCalcQueue"`
	DelayBasicLoop                 int `json:"delayBasicLoop"`
	DelayAddingLoop                int `json:"delayAddingLoop"`
	MaxGoroutinesInCalcProcess     int `json:"maxGoroutinesInCalcProcess"`
	AdditionalDelayInCalcOperation int `json:"additionalDelayInCalcOperation"`
}

func setConfigInfo(w http.ResponseWriter, r *http.Request, config *Config) {

	var updateRequest SettingsUpdateRequest
	err := json.NewDecoder(r.Body).Decode(&updateRequest)
	if err != nil {
		http.Error(w, "Error decoding JSON", http.StatusBadRequest)
		log.Printf("Error decoding JSON: %v", err)
		return
	}
	defer r.Body.Close()

	if updateRequest.Settings == "" {
		http.Error(w, "Missing 'settings' section in JSON", http.StatusBadRequest)
		log.Println("Missing 'settings' section in JSON")
		return
	}

	var settings SettingsStruct
	err = json.Unmarshal([]byte(updateRequest.Settings), &settings)
	if err != nil {
		http.Error(w, "Error decoding settings JSON", http.StatusBadRequest)
		log.Printf("Error decoding settings JSON: %v", err)
		return
	}

	// Обновление текущих настроек Демона в памяти
	// config.ManagerGRPCServer = settings.ManagerGRPCServer
	// config.ManagerGRPCServerReconnectInterval = settings.ManagerGRPCServerReconnectInterval
	config.EchoRepeatInterval = settings.EchoRepeatInterval
	config.MaxLenghtCalcQueue = settings.MaxLenghtCalcQueue
	config.DelayBasicLoop = settings.DelayBasicLoop
	config.DelayAddingLoop = settings.DelayAddingLoop
	config.MaxGoroutinesInCalcProcess = settings.MaxGoroutinesInCalcProcess
	config.AdditionalDelayInCalcOperation = settings.AdditionalDelayInCalcOperation

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// Возврат сведений о настройках и статистике Демона
func getConfigAndStatisticsInfo(w http.ResponseWriter, config *Config) {

	info := ConfigInfo{
		StartTime:         startTime.Format(time.RFC3339),
		Uptime:            int(time.Since(startTime).Seconds()),
		QntCurrentTasks:   calcQueueSize(),
		QntProcessedTasks: qntProcessedTasks,
		CurrentTasks:      make([]TaskInfo, 0),
		CurrentSettings:   *config,
	}

	calcQueue.Range(func(key, value interface{}) bool {
		RecordID := key.(int64)
		task := value.(*Task)

		taskInfo := TaskInfo{
			RecordID:      task.RecordID,
			TaskID:        task.TaskID,
			Expression:    task.Expression,
			RPNExpression: task.RPNexpression,
			IsFinished:    task.IsFinished,
			IsWrong:       task.IsWrong,
			Result:        strconv.FormatFloat(task.Result, 'f', -1, 64),
		}

		status, ok := taskStatus.Load(RecordID)
		if ok {
			taskInfo.Status = status.(string)
		}

		info.CurrentTasks = append(info.CurrentTasks, taskInfo)

		return true
	})

	responseJSON, err := json.Marshal(info)
	if err != nil {
		http.Error(w, "Error encoding JSON", http.StatusInternalServerError)
		log.Printf("Error encoding JSON: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseJSON)
}

func processingIncominHTTPRequest(w http.ResponseWriter, r *http.Request, config *Config) {
	switch r.Method {
	case "POST":
		setConfigInfo(w, r, config)
	case "GET":
		getConfigAndStatisticsInfo(w, config)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		log.Printf("Method Not Allowed: %s", r.Method)
		return
	}
}

// HTTP хендлер для работы с запросами конфигурации Демона
func configHandler(w http.ResponseWriter, r *http.Request, config *Config) {

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID")

	// Проверяем метод запроса, и если это OPTIONS, завершаем обработку
	if r.Method == "OPTIONS" {
		return
	}

	processingIncominHTTPRequest(w, r, config)
}

//-----------------------------

func startGRPCServer(config Config) {

	// Создание gRPC сервера
	grpcServer := grpc.NewServer()
	calcDaemonServer := NewServer(config)
	pb.RegisterCalcDaemonServer(grpcServer, calcDaemonServer)

	listener, err := net.Listen("tcp", config.ServerAddress+":"+config.ServerPort)
	if err != nil {
		log.Fatalf("Error starting gRPC listener: %v", err)
	}

	// Запуск gRPC сервера
	log.Printf("Calc_daemon gRPC server starting on %s:%s", config.ServerAddress, config.ServerPort)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve gRPC server: %v", err)
	}
}

func startHTTPServer(config *Config) {

	// Обработчик веб-запросов
	// http.HandleFunc(config.ConfigEndpoint, handler)
	http.HandleFunc(config.ConfigEndpoint, func(w http.ResponseWriter, r *http.Request) {
		configHandler(w, r, config)
	})

	// Берется порт на единицу больше, чем тот, на котором стартует gRPC сервер(для простоты используются различные порты)
	portInt, err := strconv.Atoi(config.ServerPort)
	if err != nil {
		log.Fatalf("Error converting a potr number(string to number): %v", err)
		return
	}

	httpSrvAddress := fmt.Sprintf("%s:%s", config.ServerAddress, strconv.Itoa(portInt+1))

	// Запуск HTTP сервера
	log.Printf("Calc_daemon HTTP server starting and listening on %s", httpSrvAddress)
	if err := http.ListenAndServe(httpSrvAddress, nil); err != nil {
		log.Fatalf("Failed to serve HTTP server: %v", err)
	}
}

func main() {

	startTime = time.Now()

	// Загрузка настроек приложения
	config, err := loadConfig("calcdaemon_config.json")
	if err != nil {
		log.Println("Error loading config (calcdaemon_config.json):", err)
		return
	}

	// Флаг для порта
	port := flag.String("port", config.ServerPort, "Порт для запуска HTTP-сервера")
	flag.Parse()

	config.ServerPort = *port // Обновляем значение ServerPort()
	grpcSrvAddress = fmt.Sprintf("%s:%s", config.ServerAddress, config.ServerPort)

	//-----------------------------

	// Запуск серверов gRPC(port) и HTTP(port+1)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		startGRPCServer(config)
	}()
	go func() {
		defer wg.Done()
		startHTTPServer(&config)
	}()

	//-----------------------------

	// gRPC-клиент
	grpcClient = connectToGRPCServer(config)

	// Запуск отправки Echo запросов
	go runEchoLoop(&config)

	// Осн. цикл(вычисление выражений, отправка промежуточных результатов Менеджеру)
	go runBasicDaemonLoop(&config)

	// Доп.цикл(чистка очереди выполненных, отправка результатов Менеджеру)
	go runAddingDaemonLoop(&config)

	select {}
}

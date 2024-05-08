package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"ya_lms_expression_calc_two/internal/expression_processing"

	_ "github.com/lib/pq"

	pb "ya_lms_expression_calc_two/grpc/proto_exchange"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	activeDaemons     map[string]time.Time       // Активные Демоны-агенты
	activeDaemonsInfo map[string]*pb.EchoRequest // Служебное инфо по активным Демонам-агентам
	activeDaemonsMux  sync.Mutex
)

func init() {
	activeDaemons = make(map[string]time.Time)
	activeDaemonsInfo = make(map[string]*pb.EchoRequest)
}

// Структура настроек Менеджера
type Config struct {
	GRPCServerAddress string `json:"grpcServerAddress"` // Адрес и порт gRPC сервера Менеджера

	EndpointCalcs     string `json:"endpointCalcs"`
	EndpointDaemons   string `json:"endpointDaemons"`
	HTTPServerAddress string `json:"httpServerAddress"` // Адрес и порт HTTP сервера Менеджера

	DBConnectionConfig struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		DBName   string `json:"dbName"`
	} `json:"dbConnectionConfig"`
	BasicLoopDelaySeconds int `json:"basicLoopDelaySeconds"` // Задержка(в сек.) основного цикла

	MaxLenghtManagerQueue int  `json:"maxLenghtManagerQueue"` // Максимальная длина очереди задач Менеджера
	MaxDurationForTask    int  `json:"maxDurationForTask"`    // Время(в сек.) свыше которого задача на вычисление считается просроченной(т.е. это максимальное время на решение)
	MakeRPNInManager      bool `json:"makeRPNInManager"`      // Флаг необходимости построения RPN на стороне Менеджера

	DaemonInactivityTimeout          int `json:"daemonInactivityTimeout"`          // Время(в сек.), свыше которого, Демон считается неактивным
	AutoCheckIntervalOfActiveDaemons int `json:"autoCheckIntervalOfActiveDaemons"` // Интервал(в сек.) автоматической корректировки активных Демонов
	TaskSendingToDaemonInterval      int `json:"taskSendingToDaemonInterval"`      // Задержка(в сек.) после отправки задачи Демону
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

type TasksStruct struct {
	RecordID           int64   `json:"id"`
	TaskID             int64   `json:"task_id"`
	CreateDateTime     string  `json:"create_date"`
	FinishDateTime     string  `json:"finish_date"`
	CalcDuration       int64   `json:"calc_duration"`
	Expression         string  `json:"expression"`
	RPNexpression      string  `json:"rpn_expression"`
	Comment            string  `json:"comment"`
	Status             string  `json:"status"`
	Condition          int     `json:"condition"`
	Result             float64 `json:"result"`
	IsWrong            bool    `json:"is_wrong"`
	IsFinished         bool    `json:"is_finished"`
	IsFinishedInRegTab bool    `json:"is_finished_regtab"`
	RequestID          string  `json:"request_id"`
	RegDateTime        string  `json:"reg_date"`
	CalcDaemon         string  `json:"calc_daemon"`
}

// Обновление значений атрибутов задачи в очереди(таблица calc_expr)
func updateTaskAttributes(db *sql.DB, RecordID int64, newValues map[string]interface{}) error {
	query := "UPDATE calc_expr SET "
	params := make([]interface{}, 0)

	// Формируем SET часть запроса
	setValues := make([]string, 0)
	for key, value := range newValues {
		setValues = append(setValues, fmt.Sprintf("%s = $%d", key, len(params)+1))
		params = append(params, value)
	}
	query += strings.Join(setValues, ", ")

	// Добавляем условие для конкретной задачи
	query += fmt.Sprintf(" WHERE id = $%d", len(params)+1)
	params = append(params, RecordID)

	// Выполняем SQL-запрос
	_, err := db.Exec(query, params...)
	if err != nil {
		log.Printf("Error updating task attributes for task whith (Record)id %d: %v", RecordID, err)
		return err
	}

	return nil
}

// Установка у старых(просроченных) задач признака завершения
func setCloseToOverdueTasks(db *sql.DB, tasks []TasksStruct) {

	for _, task := range tasks {
		newValues := map[string]interface{}{
			"is_finished": true,
			"is_wrong":    true, // т.к. задачи просрочены, то фиксируем их с ошибкой
			"condition":   4,
			"comment":     "The allowed calculation timeout has been exceeded",
			"finish_date": time.Now(),
		}
		err := updateTaskAttributes(db, task.RecordID, newValues)
		if err != nil {
			log.Printf("Error save changes for record_id %d: %v", task.RecordID, err)
		}
		continue
	}
}

// Получение просроченных задач
func getOverdueTasks(db *sql.DB, config Config) ([]TasksStruct, error) {
	var tasksList []TasksStruct

	currentTime := time.Now()

	rows, err := db.Query(`
		SELECT
			id,	
			task_id
		FROM
			calc_expr
		WHERE 
			is_finished = FALSE
			AND create_date < $1
	`, currentTime.Add(-time.Duration(config.MaxDurationForTask)*time.Second))

	// 		AND create_date < NOW() - ($1 || ' seconds')::interval
	// `, config.MaxDurationForTask)

	if err != nil {
		return nil, fmt.Errorf("error executing database query: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var task TasksStruct
		if err := rows.Scan(
			&task.RecordID,
			&task.TaskID); err != nil {
			// Логируем ошибку, но не прекращаем выполнение цикла
			log.Printf("Error scanning row: %v", err)
			continue
		}
		tasksList = append(tasksList, task)
	}

	if err := rows.Err(); err != nil {
		// Возвращаем ошибку вместе с пустым списком задач
		return nil, fmt.Errorf("error iterating over rows: %v", err)
	}

	return tasksList, nil
}

// Завершение старых(просроченных) задач
func closingOverdueTasks(db *sql.DB, config Config) error {

	if config.MaxDurationForTask > 0 {

		overdueTasks, err := getOverdueTasks(db, config)
		if err != nil {
			return err
		}

		if len(overdueTasks) > 0 {
			setCloseToOverdueTasks(db, overdueTasks)
		}
	}

	return nil
}

// - - - - - - - - - -

// Получение задач для финализации
func getTasksForFinalisation(db *sql.DB) ([]TasksStruct, error) {
	var tasksList []TasksStruct

	rows, err := db.Query(`
		SELECT
			ce.id,	
			ce.task_id,
			CASE 
                WHEN ce.finish_date IS NOT NULL AND ce.create_date IS NOT NULL THEN
                    CASE WHEN ce.finish_date < ce.create_date THEN 0
                    ELSE CAST(EXTRACT(EPOCH FROM (ce.finish_date - ce.create_date)) AS INTEGER) END 
                ELSE 0
            END AS calc_duration,
			COALESCE(ce.result, '0') as result,
			ce.comment,
			ce.condition,
			ce.is_wrong,
			COALESCE(re.is_finished, false) as is_finished_regtab
		FROM
			calc_expr AS ce
		LEFT JOIN reg_expr AS re ON ce.task_id = re.id
		WHERE ce.is_finished = TRUE
	`)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var task TasksStruct
		err := rows.Scan(
			&task.RecordID,
			&task.TaskID,
			&task.CalcDuration,
			&task.Result,
			&task.Comment,
			&task.Condition,
			&task.IsWrong,
			&task.IsFinishedInRegTab)
		if err != nil {
			return nil, err
		}
		tasksList = append(tasksList, task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasksList, nil
}

// Финализация задач (в рамках транзакции)
func setFinalStatusFinishedTasks(db *sql.DB, tasks []TasksStruct) {

	for _, task := range tasks {

		// Начало транзакции
		tx, err := db.Begin()
		if err != nil {
			log.Printf("Error starting transaction for task_id %d: %v", task.TaskID, err)
			return
		}
		defer func() {
			if err := recover(); err != nil {
				// Ошибка произошла, откатываем транзакцию
				log.Println("Rolling back transaction due to error:", err)
				tx.Rollback()
			}
		}()

		// Обработка задачи
		// if task.IsFinishedInRegTab == nil {
		// 	// В reg_expr задача отсутствует, удаляем запись из calc_expr
		// 	// (ситуация маловероята, но возможна, т.к. не используется механизм внешних ключей)
		// 	_, err := tx.Exec("DELETE FROM calc_expr WHERE id = $1", task.RecordID)
		// 	if err != nil {
		// 		panic(fmt.Sprintf("Error deleting calc_expr record for task_id %d: %v", task.TaskID, err))
		// 	}
		// } else if task.IsFinishedInRegTab {

		if task.IsFinishedInRegTab {
			// В reg_expr задача завершена, удаляем запись из calс_expr
			_, err := tx.Exec("DELETE FROM calc_expr WHERE id = $1", task.RecordID)
			if err != nil {
				panic(fmt.Sprintf("Error deleting calc_expr record for task_id %d: %v", task.TaskID, err))
			}

		} else {
			// В reg_expr задача не завершена, но завершена в calс_expr

			status := "success"
			if task.IsWrong {
				status = "error"
			} else {
				switch task.Condition {
				case 0:
					status = "new"
				case 1, 2:
					status = "success"
				case 3:
					status = "error"
				case 4:
					status = "cancel"
				}
			}

			_, err := tx.Exec(`
				UPDATE reg_expr
				SET
					is_finished = true,
					is_wrong = $1,
					result = $2,
					status = $3,
					comment = $4,
					calc_duration = $5,
					finish_date = $6
				WHERE
					id = $7
			`,
				task.IsWrong,
				task.Result,
				status,
				task.Comment,
				task.CalcDuration,
				time.Now(),
				task.TaskID)

			if err != nil {
				panic(fmt.Sprintf("Error finalizing reg_expr record for task_id %d: %v", task.TaskID, err))
			}

			// Удаляем запись из calс_expr
			_, err = tx.Exec("DELETE FROM calc_expr WHERE id = $1", task.RecordID)
			if err != nil {
				panic(fmt.Sprintf("Error deleting calc_expr record for task_id %d: %v", task.TaskID, err))
			}
		}

		// Фиксируем транзакцию
		if err := tx.Commit(); err != nil {
			log.Printf("Error committing transaction for task_id %d: %v", task.TaskID, err)
			return
		}
	}
}

// Финализация задач
func finalisationFinishedTasks(db *sql.DB) error {

	currentTasks, err := getTasksForFinalisation(db)
	if err != nil {
		return err
	}

	if len(currentTasks) > 0 {
		setFinalStatusFinishedTasks(db, currentTasks)
	}

	return nil
}

// - - - - - - - - - -

// Запрос текущей длины очереди задач(количество задач в calc_expr)
func getCurrentQueueLength(db *sql.DB) (int, error) {
	var queueLength int
	err := db.QueryRow("SELECT COUNT(*) FROM calc_expr").Scan(&queueLength)
	if err != nil {
		return 0, err
	}
	return queueLength, nil
}

// Добавление новых задач в очередь(calc_expr) в количестве не более "count"
func addNewTasksInQueue(db *sql.DB, count int) error {

	query := `
        INSERT INTO calc_expr (task_id, create_date, expression)
        SELECT
			re.id AS task_id,
			$1 AS create_date,
			re.expression
        FROM
            reg_expr AS re
        LEFT JOIN
            calc_expr AS ce ON re.id = ce.task_id
        WHERE
			re.is_finished = FALSE
            AND ce.id IS NULL
		ORDER BY re.reg_date ASC 	
        LIMIT $2;`

	now := time.Now()
	_, err := db.Exec(query, now, count)
	if err != nil {
		return err
	}

	return nil
}

// Добавление новых задач на вычисление
func addNewTasksForCalculation(db *sql.DB, config Config) error {

	queueLength, err := getCurrentQueueLength(db)
	if err != nil {
		log.Printf("Error getting current queue length: %v", err)
		return err // Выходим с ошибкой (продолжать не безопасно)
	}

	// Проверка размера очереди
	MaxLenghtManagerQueue := config.MaxLenghtManagerQueue
	countNewTasks := MaxLenghtManagerQueue - queueLength
	if countNewTasks <= 0 {
		// Очередь заполнена, выходим
		return nil
	}

	// Добавляем новые задачи в очередь
	if err := addNewTasksInQueue(db, countNewTasks); err != nil {
		log.Printf("Error in query getting and adding new tasks to queue(addNewTasksInQueue): %v", err)
		return err
	}

	return nil
}

// Формирование очереди задач
func makeTasksQueue(db *sql.DB, config Config) {

	// Завершение старых задач
	if err := closingOverdueTasks(db, config); err != nil {
		log.Printf("Error occurred when closing overdue tasks(closingOverdueTasks): %v", err)
		return
	}

	// Финализация завершенных задач
	if err := finalisationFinishedTasks(db); err != nil {
		log.Printf("Error occurred when finalizing tasks(finalisationFinishedTasks): %v", err)
		return
	}

	// Добавление новых задач на вычисление
	if err := addNewTasksForCalculation(db, config); err != nil {
		log.Printf("Error adding new tasks in queue(addNewTasksForCalculation): %v", err)
		return
	}
}

//- - - - - - - - - - - - - - - - - - - - - - - - - - - - -

// Получение незавершенных задач по значению "condition"
func getTasksByCondition(db *sql.DB, condition int) ([]TasksStruct, error) {
	var tasksList []TasksStruct

	query := `
		SELECT 
			id,
			task_id,
			expression,
			COALESCE(rpn_expression, '') AS rpn_expression,
			COALESCE(calc_daemon, '') AS calc_daemon
		FROM calc_expr
		WHERE 
			is_finished=FALSE  
			AND condition=$1 
		ORDER BY create_date ASC
	`

	rows, err := db.Query(query, condition)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var task TasksStruct
		err := rows.Scan(&task.RecordID, &task.TaskID, &task.Expression, &task.RPNexpression, &task.CalcDaemon)
		if err != nil {
			return nil, err
		}
		tasksList = append(tasksList, task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasksList, nil
}

// Обработка новых задач очереди(is_finished=FALSE AND condition=0)
func newTasksPreprocessing(db *sql.DB, config Config) {

	tasks, err := getTasksByCondition(db, 0)
	if err != nil {
		log.Printf("Error getting new tasks for pre-processing(tasks with condition = 0): %v", err)
		return
	}

	if len(tasks) == 0 {
		return
	}

	// Валидируем выражения из задач
	for _, task := range tasks {

		currExpression := strings.TrimSpace(task.Expression)
		if currExpression == "" {

			// Завершаем задачу, как не прошедшую валидацию
			newValues := map[string]interface{}{
				"is_finished": true,
				"is_wrong":    true,
				"condition":   3,
				"comment":     "An error in your expression(is empty)",
				"finish_date": time.Now(),
			}

			err = updateTaskAttributes(db, task.RecordID, newValues)
			if err != nil {
				log.Printf("Error save changes in incorrect record_id: %d: %v", task.RecordID, err)
			}
			continue
		}

		// Формируем RPN-выражение(постфиксную запись)
		postfixStr := ""
		if config.MakeRPNInManager {
			postfix, err := expression_processing.Parse(currExpression)
			if err != nil {
				log.Printf("Error getting RPN for task_id %d: %v", task.TaskID, err)

				// Завершаем задачу, как не прошедшую валидацию
				newValues := map[string]interface{}{
					"is_finished": true,
					"is_wrong":    true,
					"condition":   3,
					"comment":     "An error in your expression(getting RPN)",
					"finish_date": time.Now(),
				}
				err = updateTaskAttributes(db, task.RecordID, newValues)
				if err != nil {
					log.Printf("Error save changes in incorrect record_id %d: %v", task.RecordID, err)
				}
				continue
			}
			postfixStr = expression_processing.PostfixToString(postfix)
		}

		// Фиксируем задачу, как готовую к обработке
		newValues := map[string]interface{}{
			"rpn_expression": postfixStr,
			"is_wrong":       false,
			"condition":      1,
		}
		err = updateTaskAttributes(db, task.RecordID, newValues)
		if err != nil {
			log.Printf("Error save changes in new task_id %d: %v", task.TaskID, err)
		}

	}
}

//- - - - - - - - - - -

// Перемешивание активных Демонов-вычислителей
func getRandomDaemonSort(listOfDaemons []string) []string {

	randSource := rand.NewSource(time.Now().UnixNano())
	randGen := rand.New(randSource)

	randomIndices := randGen.Perm(len(listOfDaemons)) // Генерируем случайную перестановку индексов

	randomSortedDaemons := make([]string, len(listOfDaemons))
	for i, randomIndex := range randomIndices {
		randomSortedDaemons[i] = listOfDaemons[randomIndex] // Создаем список с элементами в случайном порядке
	}

	return randomSortedDaemons
}

// Получение списка(копии) активных Демонов-вычислителей
func getListOfActiveDaemons() []string {
	activeDaemonsMux.Lock()
	defer activeDaemonsMux.Unlock()

	var listOfDaemons []string
	for key, _ := range activeDaemons {
		listOfDaemons = append(listOfDaemons, key)
	}
	return listOfDaemons
}

// Удаление Демона из списка
func removeDaemonFromList(list []string, daemonAddr string) []string {
	var newList []string
	for _, addr := range list {
		if addr != daemonAddr {
			newList = append(newList, addr)
		}
	}
	return newList
}

// Отправка задачи Демону(в gRPS сервис CalcDaemon.AddTask)
func sendTaskToDaemon(task TasksStruct, daemonAddr string) (string, error) {

	// Устанавливаем соединение с Демоном
	conn, err := grpc.NewClient(daemonAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", err
	}
	defer conn.Close()

	// Создаем клиент для отправки запросов Демону
	client := pb.NewCalcDaemonClient(conn)

	// Формируем запрос для отправки Демону
	req := &pb.AddTaskRequest{
		RecordId:      task.RecordID,
		TaskId:        task.TaskID,
		Expression:    task.Expression,
		RpnExpression: task.RPNexpression,
	}

	// Отправляем запрос Демону
	res, err := client.AddTask(context.TODO(), req)
	if err != nil {
		return "", err
	}

	// Возвращаем статус ответа
	return res.Status, nil
}

// Распределение задач по списку случайно отсортированных активных Демонов
func distributionTask(db *sql.DB, config Config, task TasksStruct, randomSortedDaemons []string, listOfDaemons *[]string) {

	for _, daemonAddr := range randomSortedDaemons {

		// Задержка
		time.Sleep(time.Duration(config.TaskSendingToDaemonInterval) * time.Second)

		status, err := sendTaskToDaemon(task, daemonAddr)
		if err != nil {
			log.Printf("Error sending task to daemon %s: %v", daemonAddr, err)

			// Исключаем Демона из распределения, чтобы в цикле "range tasks" не было попыток отправить ему задачи
			*listOfDaemons = removeDaemonFromList(*listOfDaemons, daemonAddr)
			continue
		}

		switch status {
		case "OK":
			// Задача успешно распределена. Фиксируем
			newValues := map[string]interface{}{
				"condition":    2,
				"calc_daemon":  daemonAddr,
				"distrib_date": time.Now(),
			}
			err := updateTaskAttributes(db, task.RecordID, newValues)
			if err != nil {
				log.Printf("Error save changes for record_id %d: %v", task.RecordID, err)
			}
			// continue taskLoop
			return
		case "FULL":
			// У этого Демона заполнена очередь, исключаем его из распределения
			*listOfDaemons = removeDaemonFromList(*listOfDaemons, daemonAddr)
		default:
			log.Printf("Unknown status received from Daemon %s: %s", daemonAddr, status)
		}
	}
}

// Распределение задач по Демонам-вычислителям
func distributionOfTasks(db *sql.DB, config Config) {

	tasks, err := getTasksByCondition(db, 1)
	if err != nil {
		log.Printf("Error getting don't distributed tasks(with condition = 1): %v", err)
		return
	}

	if len(tasks) == 0 {
		// log.Println("No tasks to distribute.")
		return
	}

	// Копируем список активных Демонов-агентов. Так безопаснее обработать итерацию распределения задач
	listOfDaemons := getListOfActiveDaemons()
	if len(listOfDaemons) == 0 {
		log.Println("No active Daemons to distribute.")
		return
	}

	for _, task := range tasks {
		randomSortedDaemons := getRandomDaemonSort(listOfDaemons)
		if len(randomSortedDaemons) == 0 {
			log.Println("No empty spaces in the queue of active Daemons.")
			return
		}

		distributionTask(db, config, task, randomSortedDaemons, &listOfDaemons)
	}
}

//- - - - - - - - - - -

// Обработка очереди задач
func processingTaskQueue(db *sql.DB, config Config) {

	// Переотправка распределенных задач по назначенным Демонам-вычислителям
	//(если Демон был перезапущен, то его очередь-кэш опустела и т.о. ему нужно повторно передать задачи)
	resendingDistributedTasks(db, config)

	// Подготовка(проверка новых задач, condition=0)
	newTasksPreprocessing(db, config)

	// Распределение "подготовленных"(condition=1) задач по активным Демонам-вычислителям
	distributionOfTasks(db, config)
}

//- - - - - - - - - - - - - - - - - - - - - - - - - - - - -

// Основной цикл обработки
func basicLoop(db *sql.DB, config Config) {

	for {
		// Формирование очереди задач
		makeTasksQueue(db, config)

		// Обработка очереди задач
		processingTaskQueue(db, config)

		// Задержка в основном цикле
		time.Sleep(time.Duration(config.BasicLoopDelaySeconds) * time.Second)
	}
}

//- - - - - - - - - - - - - - - - - - - - - - - - - - - - -

type Server struct {
	db     *sql.DB
	config Config
	pb.CalcManagerServer
}

func NewServer(db *sql.DB, config Config) *Server {
	return &Server{
		db:     db,
		config: config,
	}
}

// Обработка входящего Echo-запроса
func (s *Server) Echo(ctx context.Context, in *pb.EchoRequest) (*pb.EchoResponse, error) {

	// Пополняем сведения об активных Демонов
	activeDaemonsMux.Lock()
	defer activeDaemonsMux.Unlock()

	activeDaemons[in.Address] = time.Now()
	activeDaemonsInfo[in.Address] = in

	return &pb.EchoResponse{
		Status: "OK",
	}, nil
}

// Обработка входящего Result-запроса
func (s *Server) Result(ctx context.Context, in *pb.ResultRequest) (*pb.ResultResponse, error) {
	log.Println("Incoming Result-request with params: ", in)

	// Получим задачу из очереди(calc_expr)
	taskInfo, err := getTaskFromQueueByRecordId(s.db, in.RecordId)
	if err != nil {
		// Какая-то ошибка при получении задачи из БД.
		log.Printf("Error getting task from queue by RecordID %d: %v", in.RecordId, err)
		return &pb.ResultResponse{Status: "OK"}, nil
	}

	// Получим задачу и проверим, что она распределена
	taskDaemon := strings.TrimSpace(taskInfo.CalcDaemon)
	if taskDaemon == "" {
		// У задачи не указан Демон-исполнитель
		log.Printf("Task with RecordID %d is not assigned to any daemon", in.RecordId)
		return &pb.ResultResponse{Status: "OK"}, nil
	}

	// Проверим, что Демон из сообщения активен и на него распределена задача, которая д.б. незавершенной
	if !taskInfo.IsFinished && (taskDaemon == in.Address) && isDaemonActive(in.Address) {

		// Фиксируем полученные результаты вычисления задачи
		newValues := map[string]interface{}{
			"is_finished":    in.IsFinished,
			"is_wrong":       in.IsWrong,
			"rpn_expression": in.RpnExpression,
			"comment":        in.Comment,
			"result":         in.Result,
		}

		if in.IsFinished {
			newValues["finish_date"] = time.Now()
		}

		err := updateTaskAttributes(s.db, in.RecordId, newValues)
		if err != nil {
			log.Printf("Error save changes in incorrect record_id %d: %v", in.RecordId, err)
			return &pb.ResultResponse{Status: "ERROR"}, nil
		}

		return &pb.ResultResponse{Status: "OK"}, nil // Демон скорректирует свою очередь

	} else {
		// Ничего не фиксируем
		// log.Printf("Daemon %s is not active or not assigned to the task with RecordID %d", in.Address, in.RecordId)
		return &pb.ResultResponse{Status: "OK"}, nil
	}

}

func isDaemonActive(daemonAddress string) bool {
	activeDaemonsMux.Lock()
	defer activeDaemonsMux.Unlock()

	_, exists := activeDaemons[daemonAddress]
	return exists
}

// Получение задачи из очереди(calc_expr) по RecordID
func getTaskFromQueueByRecordId(db *sql.DB, RecordID int64) (*TasksStruct, error) {
	var task TasksStruct
	err := db.QueryRow(`
		SELECT 
			id,
			task_id,
			is_finished,
			COALESCE(calc_daemon, '') AS calc_daemon
		FROM calc_expr 
		WHERE id = $1 
	`, RecordID).Scan(
		&task.RecordID,
		&task.TaskID,
		&task.IsFinished,
		&task.CalcDaemon,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("not found task by RecordID: %d", RecordID)
		}
		return nil, fmt.Errorf("error getting task by RecordID: %d", RecordID)
	}
	return &task, nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - -

// Получение незавершенных задач, распределенных на Демона(с адресом daemonAddress)
func getTasksAssignedToDaemon(db *sql.DB, daemonAddress string) ([]TasksStruct, error) {

	// Обязательно проверяем
	daemonAddress = strings.TrimSpace(daemonAddress)
	if daemonAddress == "" {
		return nil, fmt.Errorf("invalid Daemons address(is empty)")
	}

	var tasksList []TasksStruct

	query := `
		SELECT
			id,	
			task_id
		FROM
			calc_expr
		WHERE 
			is_finished = FALSE
			AND calc_daemon = $1
	`
	rows, err := db.Query(query, daemonAddress)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var task TasksStruct
		err := rows.Scan(&task.RecordID, &task.TaskID)
		if err != nil {
			return nil, err
		}
		tasksList = append(tasksList, task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasksList, nil
}

// Очистка сведений о назначении задач на текущего Демона(для последующего перераспределения)
func clearDistribInformationOfDaemonTasks(db *sql.DB, daemonAddress string) {

	// Незавершенные задачи, распределенные на текущего Демона
	tasks, err := getTasksAssignedToDaemon(db, daemonAddress)
	if err != nil {
		// Какая-то ошибка при получении задачи из БД.
		log.Printf("Error getting tasks for clear distrib information of Daemon: %v", err)
		return
	}

	// Очищаем сведения
	for _, task := range tasks {

		newValues := map[string]interface{}{
			"calc_daemon":  "",
			"distrib_date": time.Time{}, // Пустое значение для времени
			"condition":    1,
		}

		err = updateTaskAttributes(db, task.RecordID, newValues)
		if err != nil {
			log.Printf("Error save changes(clear distrib information of Daemon) in incorrect record_id: %d: %v", task.RecordID, err)
		}
	}

}

// Метод для удаления неактивных агентов-демонов
func (s *Server) removeInactiveDaemons() {

	for {
		time.Sleep(time.Duration(s.config.AutoCheckIntervalOfActiveDaemons) * time.Second)
		activeDaemonsMux.Lock()
		for address, lastEchoTime := range activeDaemons {
			if time.Since(lastEchoTime) > time.Duration(s.config.DaemonInactivityTimeout)*time.Second {
				log.Printf("Daemon at address %s is inactive, removing from active list...", address)
				delete(activeDaemons, address)
				delete(activeDaemonsInfo, address)

				// Очищаем записи о назначении задач на текущего Демона(для последующего переназначения)
				clearDistribInformationOfDaemonTasks(s.db, address)

			}
			// else {
			// 	// Вывод информации о текущем активном демоне
			// 	log.Printf("Active daemon: address=%s, last Echo time=%s", address, lastEchoTime)
			// }
		}
		activeDaemonsMux.Unlock()
	}
}

func resendingDistributedTasks(db *sql.DB, config Config) {
	tasks, err := getTasksByCondition(db, 2)
	if err != nil {
		log.Printf("Error getting distributed tasks(with condition = 2): %v", err)
		return
	}

	for _, task := range tasks {

		taskDaemon := strings.TrimSpace(task.CalcDaemon)
		if taskDaemon != "" && isDaemonActive(taskDaemon) {

			status, err := sendTaskToDaemon(task, taskDaemon)
			if err != nil {
				log.Printf("Error resending task to daemon %s: %v", taskDaemon, err)
			}
			switch status {
			case "OK":
				// log.Printf("Successfully resending task to Daemon %s: %s", taskDaemon, status)
			default:
				// log.Printf("Unknown status received from Daemon(resending task) %s: %s", taskDaemon, status)
			}

			// Задержка после переотправки
			time.Sleep(time.Duration(config.TaskSendingToDaemonInterval) * time.Second)
		}

	}
}

//- - - - - - - - - - - - - - - - - - - - - - - - - - - - -

// Определение веб-хендлеров
// func defineWebHandlers(r *mux.Router, config Config, db *sql.DB) {
func defineWebHandlers(config Config, db *sql.DB) {

	http.HandleFunc(config.EndpointCalcs, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			return
		}

		processingOfEndpointCalcsRq(w, r, db)
	})

	http.HandleFunc(config.EndpointDaemons, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			return
		}

		processingOfEndpointDaemonsRq(w, r, config, db)
	})
}

// Обработка GET config.EndpointCalcs
func endpointCalcsGet(w http.ResponseWriter, db *sql.DB) {

	// Список задач, возвращаемых в ответе
	registeredTasks, err := getRegisteredTasks(db)
	if err != nil {
		log.Printf("Error getting list of registered tasks (getRegisteredTasks): %v", err)
		http.Error(w, "Error getting list of registered tasks", http.StatusInternalServerError)
		return
	}

	responseJSON, err := json.Marshal(registeredTasks)
	if err != nil {
		http.Error(w, "Error encoding JSON", http.StatusInternalServerError)
		log.Printf("Error encoding JSON: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseJSON)
}

// DTO для возврата списка задач HTTP сервером
type RegisteredTasksForClients struct {
	TaskID         int     `json:"task_id"`
	FinishDateTime string  `json:"finish_date"`
	CalcDuration   int64   `json:"calc_duration"`
	Expression     string  `json:"expression"`
	Comment        string  `json:"comment"`
	Status         string  `json:"status"`
	Result         float64 `json:"result"`
	IsWrong        bool    `json:"is_wrong"`
	IsFinished     bool    `json:"is_finished"`
	RequestID      string  `json:"request_id"`
	RegDateTime    string  `json:"reg_date"`
	UserLogin      string  `json:"login"`
}

// Получение списка зарегистрированных задач
func getRegisteredTasks(db *sql.DB) ([]RegisteredTasksForClients, error) {
	var tasksList []RegisteredTasksForClients

	rows, err := db.Query(`
	SELECT
		re.id AS task_id,
		re.request_id,
		COALESCE(re.expression, '') AS expression,
		COALESCE(re.result, 0) as result,
		COALESCE(re.status, '') AS status,
		re.is_wrong,
		re.is_finished,
		COALESCE(re.comment, '') AS comment,
		re.reg_date,
		COALESCE(TO_CHAR(re.finish_date, 'YYYY-MM-DD HH24:MI:SS'), 'default_value') AS finish_date,
		calc_duration,
		u.login AS login
	FROM reg_expr AS re
		LEFT JOIN users AS u ON re.user_id = u.id
	ORDER BY re.reg_date DESC
	LIMIT 100;
	`)

	if err != nil {
		return nil, fmt.Errorf("error executing database query: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var task RegisteredTasksForClients
		if err := rows.Scan(
			&task.TaskID,
			&task.RequestID,
			&task.Expression,
			&task.Result,
			&task.Status,
			&task.IsWrong,
			&task.IsFinished,
			&task.Comment,
			&task.RegDateTime,
			&task.FinishDateTime,
			&task.CalcDuration,
			&task.UserLogin); err != nil {
			// Логируем ошибку, но не прекращаем выполнение цикла
			log.Printf("Error scanning row: %v", err)
			continue
		}
		tasksList = append(tasksList, task)
	}

	if err := rows.Err(); err != nil {
		// Возвращаем ошибку вместе с пустым списком задач
		return nil, fmt.Errorf("error iterating over rows: %v", err)
	}

	return tasksList, nil
}

// Обработка config.EndpointCalcs
func processingOfEndpointCalcsRq(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	switch r.Method {
	case "GET":
		endpointCalcsGet(w, db)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		log.Printf("Method Not Allowed: %s", r.Method)
		return
	}
}

//-------

// DTO для возврата сведений по Демону
type DaemonInfo struct {
	Address      string `json:"address"`
	StartTime    string `json:"startTime"`
	Capacity     int32  `json:"capacity"`
	Queue        int32  `json:"queue"`
	LastEchoTime string `json:"lastEchoTime"`
	Delay        int    `json:"delay"`
}

func getInfoAboutActiveDaemons() []*DaemonInfo {
	activeDaemonsMux.Lock()
	defer activeDaemonsMux.Unlock()

	currentTime := time.Now()
	daemonInfoList := make([]*DaemonInfo, 0)

	for key, value := range activeDaemons {

		valueInfo := activeDaemonsInfo[key]
		if valueInfo != nil {
			lastEchoTime := value
			delay := int(currentTime.Sub(lastEchoTime).Seconds())

			info := &DaemonInfo{
				Address:      key,
				StartTime:    valueInfo.StartTime,
				Capacity:     valueInfo.Capacity,
				Queue:        valueInfo.Queue,
				LastEchoTime: lastEchoTime.Format(time.RFC3339),
				Delay:        delay,
			}
			daemonInfoList = append(daemonInfoList, info)
		}
	}

	// Сортируем daemonInfoList по полю Address
	sort.Slice(daemonInfoList, func(i, j int) bool {
		return daemonInfoList[i].Address < daemonInfoList[j].Address
	})

	return daemonInfoList
}

// Обработка GET config.EndpointDaemons
func endpointDaemonsGet(w http.ResponseWriter) {

	// Сведения по активным Демонам
	activeDaemonsInfoList := getInfoAboutActiveDaemons()

	// info := make(map[string]interface{})
	// info["activeDaemonsInfo"] = activeDaemonsInfoList

	// responseJSON, err := json.Marshal(info)
	responseJSON, err := json.Marshal(activeDaemonsInfoList)
	if err != nil {
		http.Error(w, "Error encoding JSON", http.StatusInternalServerError)
		log.Printf("Error encoding JSON: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseJSON)
}

// Обработка config.EndpointDaemons
func processingOfEndpointDaemonsRq(w http.ResponseWriter, r *http.Request, config Config, db *sql.DB) {
	switch r.Method {
	case "GET":
		endpointDaemonsGet(w)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		log.Printf("Method Not Allowed: %s", r.Method)
		return
	}
}

//- - - - - - - - - - - - - - - - - - - - - - - - - - - - -

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - -

var wg sync.WaitGroup

func main() {

	// Загрузка настроек приложения
	config, err := loadConfig("manager_config.json")
	if err != nil {
		log.Println("Error loading config (manager_config.json):", err)
		return
	}

	// Создание подключения к базе данных
	db, err := createDBConnection(config)
	if err != nil {
		log.Println("Error creating DB connection:", err)
		return
	}
	defer db.Close()

	//-----------------------------

	// Основной цикл
	wg.Add(1)
	go func() {
		defer wg.Done()
		basicLoop(db, config)
	}()

	//-----------------------------

	// Создание gRPC сервера
	grpcServer := grpc.NewServer()
	calcManagerServer := NewServer(db, config)
	pb.RegisterCalcManagerServer(grpcServer, calcManagerServer)

	listener, err := net.Listen("tcp", config.GRPCServerAddress)
	if err != nil {
		log.Fatalf("Error starting listener: %v", err)
	}

	// Запуск gRPC сервера
	go func() {
		log.Printf("Manager gRPC server starting on %s", config.GRPCServerAddress)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve gRPC server: %v", err)
		}
	}()

	//-----------------------------

	// Автоматическое удаление неактивных агентов-демонов
	if config.AutoCheckIntervalOfActiveDaemons > 0 {
		go calcManagerServer.removeInactiveDaemons()
	}

	//-----------------------------

	// Определение обработчиков веб-запросов
	defineWebHandlers(config, db)

	// Запуск HTTP сервера
	go func() {
		log.Println("Manager HTTP-server starting and listening on ", config.HTTPServerAddress)
		if err := http.ListenAndServe(config.HTTPServerAddress, nil); err != nil {
			log.Fatalf("Failed to serve HTTP-server: %v", err)
		}
	}()

	//-----------------------------

	wg.Wait()
}

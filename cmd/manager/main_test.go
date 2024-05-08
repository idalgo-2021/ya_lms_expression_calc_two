package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	pb "ya_lms_expression_calc_two/grpc/proto_exchange"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name         string
		filename     string
		expected     Config
		expectedErr  error
		setupFile    func(filename string) error // Функция для настройки теста: создание файла конфигурации
		cleanupFiles []string                    // Список файлов для удаления после завершения теста
	}{
		{
			name:     "ValidConfig",
			filename: "valid_config.json",
			expected: Config{EndpointDaemons: "value"},
			setupFile: func(filename string) error {

				// Создаем временный файл с валидной JSON конфигурацией
				file, err := os.Create(filename)
				if err != nil {
					return err
				}
				defer file.Close()

				encoder := json.NewEncoder(file)
				return encoder.Encode(Config{EndpointDaemons: "value"})
			},
			cleanupFiles: []string{"valid_config.json"},
		},
		{
			name:        "NonExistentFile",
			filename:    "non_existent_config.json",
			expectedErr: errors.New("open non_existent_config.json: no such file or directory"),
		},
		{
			name:     "InvalidJSON",
			filename: "invalid_json_config.json",
			setupFile: func(filename string) error {

				// Создаем временный файл с невалидной JSON конфигурацией
				file, err := os.Create(filename)
				if err != nil {
					return err
				}
				defer file.Close()

				_, err = file.WriteString("invalid_json")
				return err
			},
			expectedErr:  errors.New("invalid character 'i' looking for beginning of value"),
			cleanupFiles: []string{"invalid_json_config.json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Выполняем настройку теста
			if tt.setupFile != nil {
				err := tt.setupFile(tt.filename)
				if err != nil {
					t.Fatalf("setup failed: %v", err)
				}
			}

			// Выполняем тестируемую функцию
			config, err := loadConfig(tt.filename)

			// Проверяем ожидаемые результаты
			if err != nil && tt.expectedErr == nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err == nil && tt.expectedErr != nil {
				t.Fatalf("expected error: %v, got nil", tt.expectedErr)
			}
			if err != nil && err.Error() != tt.expectedErr.Error() {
				t.Fatalf("expected error: %v, got: %v", tt.expectedErr, err)
			}

			if config != tt.expected {
				t.Errorf("expected config: %+v, got: %+v", tt.expected, config)
			}

			// Выполняем очистку теста
			for _, filename := range tt.cleanupFiles {
				os.Remove(filename)
			}
		})
	}
}

func TestCreateDBConnection_Success(t *testing.T) {

	config := Config{}
	config.DBConnectionConfig.Host = "localhost"
	config.DBConnectionConfig.Port = 5432
	config.DBConnectionConfig.User = "admin"
	config.DBConnectionConfig.Password = "root"
	config.DBConnectionConfig.DBName = "postgres"

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer mockDB.Close()

	mock.ExpectPing()

	db, err := createDBConnection(config)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Проверяем, что полученное соединение с базой данных не пустое
	if db == nil {
		t.Error("Expected database connection, got nil")
	}

	// Проверяем, что все ожидаемые вызовы были выполнены
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestUpdateTaskAttributes_Success(t *testing.T) {

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("UPDATE calc_expr SET field1 = \\$1, field2 = \\$2 WHERE id = \\$3").
		WithArgs("value1", 456, 123).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = updateTaskAttributes(db, 123, map[string]interface{}{"field1": "value1", "field2": 456})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestGetOverdueTasks(t *testing.T) {

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	// Определяем ожидаемый результат из базы данных
	expectedTasks := []TasksStruct{
		{RecordID: 1, TaskID: 100},
		{RecordID: 2, TaskID: 101},
	}

	// Ожидаем выполнение запроса и возвращаем заданные строки
	mock.ExpectQuery("^SELECT").
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_id"}).
			AddRow(expectedTasks[0].RecordID, expectedTasks[0].TaskID).
			AddRow(expectedTasks[1].RecordID, expectedTasks[1].TaskID))

	tasks, err := getOverdueTasks(db, Config{MaxDurationForTask: 60})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !reflect.DeepEqual(tasks, expectedTasks) {
		t.Errorf("Unexpected tasks, got: %v, want: %v", tasks, expectedTasks)
	}

	// Проверяем выполнение ожиданий мока
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestClosingOverdueTasks(t *testing.T) {

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	// Определяем ожидаемые задачи
	overdueTasks := []TasksStruct{
		{RecordID: 1, TaskID: 100},
		{RecordID: 2, TaskID: 101},
	}

	// Ожидаем выполнение запроса на получение просроченных задач и возвращаем заданные задачи
	mock.ExpectQuery("^SELECT").
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_id"}).
			AddRow(overdueTasks[0].RecordID, overdueTasks[0].TaskID).
			AddRow(overdueTasks[1].RecordID, overdueTasks[1].TaskID))

	// Ожидаем выполнение запросов на обновление просроченных задач
	mock.ExpectExec("^UPDATE").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = closingOverdueTasks(db, Config{MaxDurationForTask: 60})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestGetTasksForFinalisation(t *testing.T) {

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	// Определяем ожидаемые задачи
	expectedTasks := []TasksStruct{
		{RecordID: 1, TaskID: 100, CalcDuration: 10, Result: 4, Comment: "", Condition: 0, IsWrong: false, IsFinishedInRegTab: true},
		{RecordID: 2, TaskID: 101, CalcDuration: 15, Result: 6, Comment: "Some comment", Condition: 1, IsWrong: true, IsFinishedInRegTab: false},
	}

	// Ожидаем выполнение запроса SELECT и возвращаем заданные задачи
	mock.ExpectQuery("^SELECT").
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "calc_duration", "result", "comment", "condition", "is_wrong", "is_finished_regtab"}).
			AddRow(expectedTasks[0].RecordID, expectedTasks[0].TaskID, expectedTasks[0].CalcDuration, expectedTasks[0].Result, expectedTasks[0].Comment, expectedTasks[0].Condition, expectedTasks[0].IsWrong, expectedTasks[0].IsFinishedInRegTab).
			AddRow(expectedTasks[1].RecordID, expectedTasks[1].TaskID, expectedTasks[1].CalcDuration, expectedTasks[1].Result, expectedTasks[1].Comment, expectedTasks[1].Condition, expectedTasks[1].IsWrong, expectedTasks[1].IsFinishedInRegTab))

	tasks, err := getTasksForFinalisation(db)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !reflect.DeepEqual(tasks, expectedTasks) {
		t.Errorf("Unexpected tasks list. Expected: %v, got: %v", expectedTasks, tasks)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestGetTasksForFinalisation_Error(t *testing.T) {

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT").
		WillReturnError(errors.New("database error"))

	_, err = getTasksForFinalisation(db)

	if err == nil || err.Error() != "database error" {
		t.Errorf("Expected database error, got: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

// ------------------

func TestGetCurrentQueueLength(t *testing.T) {

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))

	queueLength, err := getCurrentQueueLength(db)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if queueLength != 5 {
		t.Errorf("Unexpected queue length. Got: %d, Expected: %d", queueLength, 5)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestAddNewTasksInQueue(t *testing.T) {

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	// Ожидаем запрос на вставку задач в очередь.
	mock.ExpectExec("INSERT INTO calc_expr").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()). // Любые аргументы
		WillReturnResult(sqlmock.NewResult(1, 1))     // Результат вставки

	count := 5
	err = addNewTasksInQueue(db, count)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestAddNewTasksForCalculation(t *testing.T) {

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock database: %v", err)
	}
	defer db.Close()

	config := Config{MaxLenghtManagerQueue: 10}

	// Ожидаем запрос на получение текущей длины очереди
	rows := sqlmock.NewRows([]string{"queueLength"}).AddRow(5)
	mock.ExpectQuery("SELECT COUNT").WillReturnRows(rows)

	mock.ExpectExec("INSERT INTO calc_expr").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))

	err = addNewTasksForCalculation(db, config)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

//-----------------

// func TestGetTasksByCondition(t *testing.T) {
//
// 	db, mock, err := sqlmock.New()
// 	if err != nil {
// 		t.Fatalf("Ошибка создания макета базы данных: %v", err)
// 	}
// 	defer db.Close()
//
// 	currentTime := time.Now().Format(time.RFC3339)
// 	expectedTasks := []TasksStruct{
// 		{RecordID: 1, TaskID: 101, Expression: "2+2", RPNexpression: "2 2 +", CalcDaemon: "", Condition: 1, IsFinished: false, CreateDateTime: currentTime},
// 		{RecordID: 2, TaskID: 102, Expression: "3*3", RPNexpression: "3 3 *", CalcDaemon: "", Condition: 1, IsFinished: false, CreateDateTime: currentTime},
// 	}
//
// 	// Ожидаемый запрос
// 	expectedQuery := regexp.QuoteMeta("SELECT id, task_id, expression, COALESCE(rpn_expression, '') AS rpn_expression, COALESCE(calc_daemon, '') AS calc_daemon FROM calc_expr WHERE is_finished=FALSE AND condition=$1 ORDER BY create_date ASC")
// 	mock.ExpectQuery(expectedQuery).
// 		WithArgs(1).
// 		WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "expression", "rpn_expression", "calc_daemon"}).
// 			AddRow(1, 101, "2+2", "2 2 +", "").
// 			AddRow(2, 102, "3*3", "3 3 *", ""))
//
// 	tasks, err := getTasksByCondition(db, 1)
// 	if err != nil {
// 		t.Fatalf("Ошибка выполнения запроса: %v", err)
// 	}
//
// 	if len(tasks) != len(expectedTasks) {
// 		t.Fatalf("Ожидается %d задач, получено %d", len(expectedTasks), len(tasks))
// 	}
//
// 	// for i, task := range tasks {
// 	// 	expectedTask := expectedTasks[i]
// 	// 	if !reflect.DeepEqual(task, expectedTask) {
// 	// 		t.Errorf("Несоответствие ожидаемой задаче по индексу %d. Ожидается %v, получено %v", i, expectedTask, task)
// 	// 	}
// 	// }
//
// 	if err := mock.ExpectationsWereMet(); err != nil {
// 		t.Errorf("Ожидания не выполнены: %v", err)
// 	}
// }

//-----------------

func TestGetRandomDaemonSort(t *testing.T) {

	listOfDaemons := []string{"daemon1", "daemon2", "daemon3", "daemon4"}

	result := getRandomDaemonSort(listOfDaemons)

	if len(result) != len(listOfDaemons) {
		t.Errorf("Length of result does not match the length of input list")
	}

	isPermutation := true
	for _, daemon := range listOfDaemons {
		if !contains(result, daemon) {
			isPermutation = false
			break
		}
	}

	if !isPermutation {
		t.Errorf("Result is not a permutation of the input list")
	}
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

func TestGetListOfActiveDaemons(t *testing.T) {

	activeDaemonsMux.Lock()
	activeDaemons = map[string]time.Time{
		"daemon1": time.Now(),
		"daemon2": time.Now(),
		"daemon3": time.Now(),
	}
	activeDaemonsMux.Unlock()

	result := getListOfActiveDaemons()

	expectedLength := 3
	if len(result) != expectedLength {
		t.Errorf("Длина возвращенного списка не соответствует ожидаемой. Ожидается %d, получено %d", expectedLength, len(result))
	}

	expectedDaemons := []string{"daemon1", "daemon2", "daemon3"}
	for _, daemon := range expectedDaemons {
		if !contains(result, daemon) {
			t.Errorf("Ожидаемый демон %s отсутствует в возвращенном списке", daemon)
		}
	}
}

func TestRemoveDaemonFromList(t *testing.T) {

	daemonList := []string{"daemon1", "daemon2", "daemon3"}
	daemonToRemove := "daemon2"

	result := removeDaemonFromList(daemonList, daemonToRemove)

	if contains(result, daemonToRemove) {
		t.Errorf("Демон %s не был удален из списка", daemonToRemove)
	}

	expectedLength := len(daemonList) - 1
	if len(result) != expectedLength {
		t.Errorf("Длина возвращенного списка не соответствует ожидаемой. Ожидается %d, получено %d", expectedLength, len(result))
	}

	for _, daemon := range daemonList {
		if daemon != daemonToRemove && !contains(result, daemon) {
			t.Errorf("Демон %s отсутствует в возвращенном списке", daemon)
		}
	}
}

//-----------

type mockCalcDaemonClient struct{}

func (m *mockCalcDaemonClient) AddTask(ctx context.Context, req *pb.AddTaskRequest, opts ...grpc.CallOption) (*pb.AddTaskResponse, error) {

	//
	// В этой функции мы можем возвращать фиксированный статус и ошибку или даже что-то более специфичное для наших тестов.
	// return &pb.AddTaskResponse{Status: "Success"}, nil
	return nil, nil
}

func TestSendTaskToDaemon(t *testing.T) {

	// Тест работает, если на соотв.адресе работает Демон. Как это всё тестить на моках, я ещё не разобрался

	mockClient := &mockCalcDaemonClient{}
	fmt.Println(mockClient)

	task := TasksStruct{
		RecordID:      1,
		TaskID:        101,
		Expression:    "2+2",
		RPNexpression: "2 2 +",
	}

	daemonAddr := "localhost:9000"
	_, err := sendTaskToDaemon(task, daemonAddr)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// if status != "OK" {
	// 	t.Errorf("Неверный статус ответа: ожидался 'Success', получен '%s'", status)
	// }
}

//-----------

func TestGetTaskFromQueueByRecordId_Success(t *testing.T) {

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	recordID := int64(1)

	expectedTask := &TasksStruct{
		RecordID:   1,
		TaskID:     101,
		IsFinished: false,
		CalcDaemon: "localhost:9000",
	}

	rows := sqlmock.NewRows([]string{"id", "task_id", "is_finished", "calc_daemon"}).
		AddRow(expectedTask.RecordID, expectedTask.TaskID, expectedTask.IsFinished, expectedTask.CalcDaemon)
	mock.ExpectQuery("^SELECT").
		WithArgs(recordID).
		WillReturnRows(rows)

	task, err := getTaskFromQueueByRecordId(db, recordID)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if task == nil {
		t.Error("Expected non-nil task")
	} else {
		if *task != *expectedTask {
			t.Errorf("Returned task does not match expected task. Got: %+v, Expected: %+v", task, expectedTask)
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}

}

func TestGetTaskFromQueueByRecordId_RecordNotFound(t *testing.T) {

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	recordIDNotFound := int64(999)
	expectedErrorMsg := fmt.Sprintf("not found task by RecordID: %d", recordIDNotFound)

	mock.ExpectQuery("^SELECT").
		WithArgs(recordIDNotFound).
		WillReturnError(sql.ErrNoRows)

	taskNotFound, err := getTaskFromQueueByRecordId(db, recordIDNotFound)
	if err == nil || err.Error() != expectedErrorMsg {
		t.Errorf("Expected error message '%s', but got: %v", expectedErrorMsg, err)
	}
	if taskNotFound != nil {
		t.Errorf("Expected nil task, but got: %+v", taskNotFound)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestGetTaskFromQueueByRecordId_ErrorQuery(t *testing.T) {

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	recordID := int64(1)
	expectedErrorMsg := fmt.Sprintf("error getting task by RecordID: %d", recordID)

	mock.ExpectQuery("^SELECT").
		WithArgs(recordID).
		WillReturnError(fmt.Errorf("some database error"))

	taskWithError, err := getTaskFromQueueByRecordId(db, recordID)
	if err == nil || err.Error() != expectedErrorMsg {
		t.Errorf("Expected error message '%s', but got: %v", expectedErrorMsg, err)
	}
	if taskWithError != nil {
		t.Errorf("Expected nil task, but got: %+v", taskWithError)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestGetTasksAssignedToDaemon_Success(t *testing.T) {

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	daemonAddress := "localhost:9000"
	expectedTasks := []TasksStruct{
		{RecordID: 1, TaskID: 101},
		{RecordID: 2, TaskID: 102},
	}

	rows := sqlmock.NewRows([]string{"id", "task_id"}).
		AddRow(expectedTasks[0].RecordID, expectedTasks[0].TaskID).
		AddRow(expectedTasks[1].RecordID, expectedTasks[1].TaskID)

	mock.ExpectQuery("^SELECT").
		WithArgs(daemonAddress).
		WillReturnRows(rows)

	tasks, err := getTasksAssignedToDaemon(db, daemonAddress)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(tasks) != len(expectedTasks) {
		t.Errorf("Unexpected number of tasks. Got: %d, Expected: %d", len(tasks), len(expectedTasks))
	}

	for i, task := range tasks {
		if task != expectedTasks[i] {
			t.Errorf("Returned task does not match expected task. Got: %+v, Expected: %+v", task, expectedTasks[i])
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestGetTasksAssignedToDaemon_InvalidAddress(t *testing.T) {

	db, _, _ := sqlmock.New()
	defer db.Close()

	daemonAddress := ""

	tasks, err := getTasksAssignedToDaemon(db, daemonAddress)
	if err == nil {
		t.Error("Expected error, but got nil")
	}
	if tasks != nil {
		t.Errorf("Expected nil tasks, but got: %+v", tasks)
	}
}

func TestGetTasksAssignedToDaemon_ErrorQuery(t *testing.T) {

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	daemonAddress := "localhost:9000"

	mock.ExpectQuery("^SELECT").
		WithArgs(daemonAddress).
		WillReturnError(fmt.Errorf("some database error"))

	tasks, err := getTasksAssignedToDaemon(db, daemonAddress)
	if err == nil {
		t.Error("Expected error, but got nil")
	}
	if tasks != nil {
		t.Errorf("Expected nil tasks, but got: %+v", tasks)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

//-----------

func TestGetRegisteredTasks_Success(t *testing.T) {

	db, mock, _ := sqlmock.New()
	defer db.Close()

	expectedTasks := []RegisteredTasksForClients{
		{TaskID: 1, RequestID: "101", Expression: "expression1", Result: 10.5, Status: "status1", IsWrong: false, IsFinished: false, Comment: "comment1", RegDateTime: "2024-04-23 10:00:00", FinishDateTime: "default_value", CalcDuration: 5, UserLogin: "user1"},
		{TaskID: 2, RequestID: "102", Expression: "expression2", Result: 20.5, Status: "status2", IsWrong: true, IsFinished: true, Comment: "comment2", RegDateTime: "2024-04-22 11:00:00", FinishDateTime: "2024-04-23 12:00:00", CalcDuration: 10, UserLogin: "user2"},
	}

	rows := sqlmock.NewRows([]string{"task_id", "request_id", "expression", "result", "status", "is_wrong", "is_finished", "comment", "reg_date", "finish_date", "calc_duration", "login"}).
		AddRow(expectedTasks[0].TaskID, expectedTasks[0].RequestID, expectedTasks[0].Expression, expectedTasks[0].Result, expectedTasks[0].Status, expectedTasks[0].IsWrong, expectedTasks[0].IsFinished, expectedTasks[0].Comment, expectedTasks[0].RegDateTime, expectedTasks[0].FinishDateTime, expectedTasks[0].CalcDuration, expectedTasks[0].UserLogin).
		AddRow(expectedTasks[1].TaskID, expectedTasks[1].RequestID, expectedTasks[1].Expression, expectedTasks[1].Result, expectedTasks[1].Status, expectedTasks[1].IsWrong, expectedTasks[1].IsFinished, expectedTasks[1].Comment, expectedTasks[1].RegDateTime, expectedTasks[1].FinishDateTime, expectedTasks[1].CalcDuration, expectedTasks[1].UserLogin)

	mock.ExpectQuery("^SELECT").
		WillReturnRows(rows)

	tasks, err := getRegisteredTasks(db)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !reflect.DeepEqual(tasks, expectedTasks) {
		t.Errorf("Returned tasks do not match expected tasks. Got: %+v, Expected: %+v", tasks, expectedTasks)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestGetRegisteredTasks_ErrorExecutingQuery(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()

	mock.ExpectQuery("^SELECT").
		WillReturnError(fmt.Errorf("some database error"))

	_, err := getRegisteredTasks(db)
	if err == nil {
		t.Error("Expected error, but got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestGetRegisteredTasks_ErrorScanningRows(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()

	rows := sqlmock.NewRows([]string{"task_id", "request_id", "expression", "result", "status", "is_wrong", "is_finished", "comment", "reg_date", "finish_date", "calc_duration", "login"}).
		AddRow(1, "101", "expression1", 10.5, "status1", false, false, "comment1", "2024-04-23 10:00:00", "default_value", 5, "user1").
		AddRow(2, "102", "expression2", 20.5, "status2", true, true, "comment2", "2024-04-22 11:00:00", "2024-04-23 12:00:00", 10, "user2").
		RowError(1, fmt.Errorf("some scanning error"))

	mock.ExpectQuery("^SELECT").
		WillReturnRows(rows)

	_, err := getRegisteredTasks(db)
	if err == nil {
		t.Error("Expected error, but got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

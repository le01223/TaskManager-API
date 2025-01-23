package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

var db *sql.DB

type Task struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	DueDate     time.Time `json:"dueDate"`
	Tags        string    `json:"tags"`
}

func main() {
	var err error
	db, err = sql.Open("postgres", "user=yourusername password=yourpassword dbname=yourdbname sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	router := gin.Default()

	router.POST("/task", createTask)
	router.GET("/task/:taskId", getOneTaskById)
	router.DELETE("task/:taskId", deleteTask)
	router.GET("/due/:yy/:mm/:dd", getTasksByDueDate)

	router.Run()
}

func createTask(c *gin.Context) {
	var newTask Task
	if err := c.ShouldBindJSON(&newTask); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Невалидный JSON"})
		return
	}
	err := createTaskInDB(db, &newTask)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": newTask.ID})
}

func createTaskInDB(db *sql.DB, task *Task) error {
	sqlStatement := "INSERT INTO tasks (title, description, due_date, tags) VALUES ($1, $2, $3, $4) RETURNING id;"
	err := db.QueryRow(sqlStatement, task.Title, task.Description, task.DueDate, task.Tags).Scan(&task.ID)
	if err != nil {
		return fmt.Errorf("ошибка при создании задачи: %w", err)
	}
	return nil
}
func getOneTaskById(c *gin.Context) {
	id := c.Param("taskId")
	sqlStatement := "SELECT id, title, description, due_date, tags FROM tasks WHERE id = $1"
	row := db.QueryRow(sqlStatement, id)
	var task Task
	err := row.Scan(&task.ID, &task.Title, &task.Description, &task.DueDate, &task.Tags)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Нет такого id'шника"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при чтении"})
		return
	}
	c.JSON(http.StatusOK, task)
}
func deleteTask(c *gin.Context) {
	ctx := context.Background()
	taskId := c.Param("taskId")
	affectedRows, err := deleteTaskFromDB(ctx, db, taskId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при чтении"})
		return
	}

	if affectedRows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "задача не найдена"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"msg": "ОК"})
}

func deleteTaskFromDB(ctx context.Context, db *sql.DB, taskId string) (int64, error) {
	sqlStatement := "DELETE FROM tasks WHERE id = $1"
	stmt, err := db.PrepareContext(ctx, sqlStatement)
	if err != nil {
		return 0, fmt.Errorf("error %w", err)
	}
	defer stmt.Close()

	result, err := stmt.ExecContext(ctx, taskId)
	if err != nil {
		return 0, fmt.Errorf("error  %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("error %w", err)
	}
	return rowsAffected, nil
}
func getTasksByDueDate(c *gin.Context) {
	ctx := context.Background()
	yearStr := c.Param("yy")
	monthStr := c.Param("mm")
	dayStr := c.Param("dd")

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Невалидные данные(год)")
		return
	}
	month, err := strconv.Atoi(monthStr)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Невалидные данные(месяц)")
		return
	}
	day, err := strconv.Atoi(dayStr)
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Невалидные данные(день)")
		return
	}

	dueDate, err := time.Parse("2006-01-02", fmt.Sprintf("%d-%02d-%02d", year, month, day))
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "Невалидный формат даты")
		return
	}

	tasks, err := getTasksByDueDateFromDB(ctx, db, dueDate)
	if err != nil {
		log.Println("Ошибка:", err)
		respondWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	if len(tasks) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"msg": "Нет задач на этот день"})
		return
	}

	c.JSON(http.StatusOK, tasks)
}

func respondWithError(c *gin.Context, code int, message string) {
	c.JSON(code, gin.H{"error": message})
}
func getTasksByDueDateFromDB(ctx context.Context, db *sql.DB, dueDate time.Time) ([]Task, error) {
	sqlStatement := `SELECT id, title, description, due_date, tags FROM tasks WHERE due_date = $1;`
	rows, err := db.QueryContext(ctx, sqlStatement, dueDate)
	if err != nil {
		return nil, fmt.Errorf("Ошибка: %w", err)
	}
	defer rows.Close()

	tasks := []Task{}
	for rows.Next() {
		var task Task
		if err := rows.Scan(&task.ID, &task.Title, &task.Description, &task.DueDate, &task.Tags); err != nil {
			return nil, fmt.Errorf("Ошибка: %w", err)
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("Ошибка %w", err)
	}

	return tasks, nil
}

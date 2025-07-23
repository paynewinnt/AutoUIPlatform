package handlers

import (
	"autoui-platform/backend/internal/models"
	"autoui-platform/backend/pkg/database"
	"autoui-platform/backend/pkg/response"
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetExecutions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	status := c.Query("status")

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}

	var executions []models.TestExecution
	var total int64

	query := database.DB.Model(&models.TestExecution{})
	if status != "" {
		query = query.Where("status = ?", status)
	}

	// Count total
	query.Count(&total)

	// Get paginated executions with relations
	offset := (page - 1) * pageSize
	err := query.Preload("TestCase").Preload("TestSuite").Preload("User").
		Order("created_at DESC").
		Offset(offset).Limit(pageSize).Find(&executions).Error
	if err != nil {
		response.InternalServerError(c, "获取执行记录失败")
		return
	}

	// Clear user passwords
	for i := range executions {
		executions[i].User.Password = ""
	}

	response.Page(c, executions, total, page, pageSize)
}

func GetExecution(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的执行记录ID")
		return
	}

	var execution models.TestExecution
	err = database.DB.Preload("TestCase").Preload("TestCase.Project").
		Preload("TestCase.Environment").Preload("TestCase.Device").
		Preload("TestSuite").Preload("User").
		First(&execution, id).Error
	if err != nil {
		response.NotFound(c, "执行记录不存在")
		return
	}

	execution.User.Password = ""
	response.Success(c, execution)
}

func DeleteExecution(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的执行记录ID")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var execution models.TestExecution
	err = database.DB.Where("id = ? AND user_id = ?", id, userID).First(&execution).Error
	if err != nil {
		response.NotFound(c, "执行记录不存在或无权限")
		return
	}

	// Don't allow deleting running executions
	if execution.Status == "running" || execution.Status == "pending" {
		response.BadRequest(c, "不能删除正在运行的执行记录")
		return
	}

	// Delete related performance metrics first
	database.DB.Where("execution_id = ?", id).Delete(&models.PerformanceMetric{})

	// Delete related screenshots
	database.DB.Where("execution_id = ?", id).Delete(&models.Screenshot{})

	// Delete execution record
	err = database.DB.Delete(&execution).Error
	if err != nil {
		response.InternalServerError(c, "删除执行记录失败")
		return
	}

	response.SuccessWithMessage(c, "删除成功", nil)
}

func GetExecutionLogs(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的执行记录ID")
		return
	}

	var execution models.TestExecution
	err = database.DB.Select("execution_logs").First(&execution, id).Error
	if err != nil {
		response.NotFound(c, "执行记录不存在")
		return
	}

	// Parse logs JSON
	var logs []map[string]interface{}
	if execution.ExecutionLogs != "" && execution.ExecutionLogs != "[]" {
		err = json.Unmarshal([]byte(execution.ExecutionLogs), &logs)
		if err != nil {
			response.InternalServerError(c, "解析执行日志失败")
			return
		}
	}

	response.Success(c, gin.H{
		"logs": logs,
	})
}

func GetExecutionScreenshots(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的执行记录ID")
		return
	}

	// Get screenshots from database
	var screenshots []models.Screenshot
	err = database.DB.Where("execution_id = ?", id).Order("step_index ASC").Find(&screenshots).Error
	if err != nil {
		response.InternalServerError(c, "获取截图记录失败")
		return
	}

	// Also get screenshots from execution record
	var execution models.TestExecution
	err = database.DB.Select("screenshots").First(&execution, id).Error
	if err != nil {
		response.NotFound(c, "执行记录不存在")
		return
	}

	// Parse screenshots JSON from execution record
	var executionScreenshots []string
	if execution.Screenshots != "" && execution.Screenshots != "[]" {
		err = json.Unmarshal([]byte(execution.Screenshots), &executionScreenshots)
		if err != nil {
			response.InternalServerError(c, "解析截图数据失败")
			return
		}
	}

	response.Success(c, gin.H{
		"screenshots":           screenshots,
		"execution_screenshots": executionScreenshots,
	})
}

func StopExecution(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的执行记录ID")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var execution models.TestExecution
	err = database.DB.Where("id = ? AND user_id = ?", id, userID).First(&execution).Error
	if err != nil {
		response.NotFound(c, "执行记录不存在或无权限")
		return
	}

	// Only allow stopping running or pending executions
	if execution.Status != "running" && execution.Status != "pending" {
		response.BadRequest(c, "只能停止运行中或等待中的执行记录")
		return
	}

	// Update execution status to cancelled
	err = database.DB.Model(&execution).Updates(models.TestExecution{
		Status: "cancelled",
	}).Error
	if err != nil {
		response.InternalServerError(c, "停止执行失败")
		return
	}

	response.SuccessWithMessage(c, "停止执行成功", nil)
}
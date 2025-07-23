package handlers

import (
	"autoui-platform/backend/internal/models"
	"autoui-platform/backend/pkg/database"
	"autoui-platform/backend/pkg/response"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func GetReports(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	projectID := c.Query("project_id")

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}

	var reports []models.TestReport
	var total int64

	query := database.DB.Model(&models.TestReport{})
	if projectID != "" {
		query = query.Where("project_id = ?", projectID)
	}

	// Count total
	query.Count(&total)

	// Get paginated reports with relations
	offset := (page - 1) * pageSize
	err := query.Preload("Project").Preload("TestSuite").Preload("User").
		Order("created_at DESC").
		Offset(offset).Limit(pageSize).Find(&reports).Error
	if err != nil {
		response.InternalServerError(c, "获取测试报告失败")
		return
	}

	// Clear user passwords
	for i := range reports {
		reports[i].User.Password = ""
	}

	response.Page(c, reports, total, page, pageSize)
}

func GetReport(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的报告ID")
		return
	}

	var report models.TestReport
	err = database.DB.Preload("Project").Preload("TestSuite").Preload("User").
		Preload("Executions").Preload("Executions.TestCase").
		First(&report, id).Error
	if err != nil {
		response.NotFound(c, "测试报告不存在")
		return
	}

	report.User.Password = ""
	response.Success(c, report)
}

func CreateReport(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req struct {
		Name         string `json:"name" binding:"required,min=1,max=200"`
		ProjectID    uint   `json:"project_id" binding:"required"`
		TestSuiteID  *uint  `json:"test_suite_id"`
		ExecutionIDs []uint `json:"execution_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Verify project exists and user has permission
	var project models.Project
	err := database.DB.Where("id = ? AND user_id = ? AND status = ?", req.ProjectID, userID, 1).
		First(&project).Error
	if err != nil {
		response.NotFound(c, "项目不存在或无权限")
		return
	}

	// Verify test suite if provided
	if req.TestSuiteID != nil {
		var testSuite models.TestSuite
		err := database.DB.Where("id = ? AND project_id = ? AND status = ?", 
			*req.TestSuiteID, req.ProjectID, 1).First(&testSuite).Error
		if err != nil {
			response.NotFound(c, "测试套件不存在或不属于该项目")
			return
		}
	}

	// Verify executions exist and calculate statistics
	var executions []models.TestExecution
	err = database.DB.Where("id IN ?", req.ExecutionIDs).Find(&executions).Error
	if err != nil || len(executions) != len(req.ExecutionIDs) {
		response.BadRequest(c, "部分执行记录不存在")
		return
	}

	// Calculate statistics
	var totalCases, passedCases, failedCases, errorCases int
	var minStartTime, maxEndTime time.Time
	var totalDuration int

	for i, execution := range executions {
		totalCases++
		
		switch execution.Status {
		case "passed":
			passedCases++
		case "failed":
			failedCases++
		case "error":
			errorCases++
		}

		if i == 0 || execution.StartTime.Before(minStartTime) {
			minStartTime = execution.StartTime
		}

		if execution.EndTime != nil {
			if i == 0 || execution.EndTime.After(maxEndTime) {
				maxEndTime = *execution.EndTime
			}
		}

		totalDuration += execution.Duration
	}

	// Set default end time if no executions have end time
	if maxEndTime.IsZero() {
		maxEndTime = time.Now()
	}

	// Calculate total duration
	reportDuration := int(maxEndTime.Sub(minStartTime).Seconds())
	if reportDuration <= 0 {
		reportDuration = totalDuration
	}

	// Determine status
	status := "completed"
	for _, execution := range executions {
		if execution.Status == "running" || execution.Status == "pending" {
			status = "running"
			break
		}
	}

	// Create report
	report := models.TestReport{
		Name:        req.Name,
		ProjectID:   req.ProjectID,
		TestSuiteID: req.TestSuiteID,
		TotalCases:  totalCases,
		PassedCases: passedCases,
		FailedCases: failedCases,
		ErrorCases:  errorCases,
		StartTime:   minStartTime,
		EndTime:     maxEndTime,
		Duration:    reportDuration,
		Status:      status,
		UserID:      userID.(uint),
	}

	err = database.DB.Create(&report).Error
	if err != nil {
		response.InternalServerError(c, "创建测试报告失败")
		return
	}

	// Associate executions with report
	err = database.DB.Model(&report).Association("Executions").Replace(executions)
	if err != nil {
		response.InternalServerError(c, "关联执行记录失败")
		return
	}

	// Load relations for response
	database.DB.Preload("Project").Preload("TestSuite").Preload("User").
		Preload("Executions").First(&report, report.ID)
	report.User.Password = ""

	response.SuccessWithMessage(c, "创建成功", report)
}

func DeleteReport(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的报告ID")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var report models.TestReport
	err = database.DB.Where("id = ? AND user_id = ?", id, userID).First(&report).Error
	if err != nil {
		response.NotFound(c, "测试报告不存在或无权限")
		return
	}

	// Remove execution associations first
	err = database.DB.Model(&report).Association("Executions").Clear()
	if err != nil {
		response.InternalServerError(c, "清除执行记录关联失败")
		return
	}

	// Delete report
	err = database.DB.Delete(&report).Error
	if err != nil {
		response.InternalServerError(c, "删除测试报告失败")
		return
	}

	response.SuccessWithMessage(c, "删除成功", nil)
}
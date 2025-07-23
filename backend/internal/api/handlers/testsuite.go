package handlers

import (
	"autoui-platform/backend/internal/executor"
	"autoui-platform/backend/internal/models"
	"autoui-platform/backend/pkg/database"
	"autoui-platform/backend/pkg/response"
	"encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func GetTestSuites(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	projectID := c.Query("project_id")

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}

	var testSuites []models.TestSuite
	var total int64

	query := database.DB.Model(&models.TestSuite{}).Where("status = ?", 1)
	if projectID != "" {
		query = query.Where("project_id = ?", projectID)
	}

	// Count total
	query.Count(&total)

	// Get paginated test suites with relations
	offset := (page - 1) * pageSize
	err := query.Preload("Project").Preload("Environment").Preload("User").Preload("TestCases").
		Offset(offset).Limit(pageSize).Find(&testSuites).Error
	if err != nil {
		response.InternalServerError(c, "获取测试套件列表失败")
		return
	}

	// Clear user passwords and set test case counts
	for i := range testSuites {
		testSuites[i].User.Password = ""
		testSuites[i].TestCaseCount = len(testSuites[i].TestCases)
	}

	response.Page(c, testSuites, total, page, pageSize)
}

func CreateTestSuite(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req struct {
		Name            string `json:"name" binding:"required,min=1,max=200"`
		Description     string `json:"description" binding:"max=1000"`
		ProjectID       uint   `json:"project_id" binding:"required"`
		EnvironmentID   uint   `json:"environment_id" binding:"required"`
		TestCaseIDs     []uint `json:"test_case_ids"`
		Tags            string `json:"tags" binding:"max=500"`
		Priority        int    `json:"priority" binding:"min=1,max=3"`
		CronExpression  string `json:"cron_expression" binding:"max=100"`
		IsParallel      bool   `json:"is_parallel"`
		TimeoutMinutes  int    `json:"timeout_minutes" binding:"min=1,max=1440"`
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

	// Verify environment exists
	var environment models.Environment
	err = database.DB.Where("id = ? AND status = ?", req.EnvironmentID, 1).First(&environment).Error
	if err != nil {
		response.NotFound(c, "环境不存在")
		return
	}

	// Check if test suite name exists in the project
	var existingTestSuite models.TestSuite
	err = database.DB.Where("name = ? AND project_id = ? AND status = ?", req.Name, req.ProjectID, 1).
		First(&existingTestSuite).Error
	if err == nil {
		response.BadRequest(c, "测试套件名称在该项目中已存在")
		return
	}

	// Verify test cases exist and belong to the same project
	var testCases []models.TestCase
	if len(req.TestCaseIDs) > 0 {
		err = database.DB.Where("id IN ? AND project_id = ? AND status = ?", req.TestCaseIDs, req.ProjectID, 1).
			Find(&testCases).Error
		if err != nil || len(testCases) != len(req.TestCaseIDs) {
			response.BadRequest(c, "部分测试用例不存在或不属于该项目")
			return
		}
	}

	testSuite := models.TestSuite{
		Name:           req.Name,
		Description:    req.Description,
		ProjectID:      req.ProjectID,
		EnvironmentID:  req.EnvironmentID,
		Tags:           req.Tags,
		Priority:       req.Priority,
		CronExpression: req.CronExpression,
		IsParallel:     req.IsParallel,
		TimeoutMinutes: req.TimeoutMinutes,
		Status:         1,
		UserID:         userID.(uint),
		TestCases:      testCases,
	}

	err = database.DB.Create(&testSuite).Error
	if err != nil {
		response.InternalServerError(c, "创建测试套件失败")
		return
	}

	// Associate test cases
	if len(testCases) > 0 {
		err = database.DB.Model(&testSuite).Association("TestCases").Replace(testCases)
		if err != nil {
			response.InternalServerError(c, "关联测试用例失败")
			return
		}
	}

	// Load relations for response
	database.DB.Preload("Project").Preload("Environment").Preload("User").Preload("TestCases").
		First(&testSuite, testSuite.ID)
	testSuite.User.Password = ""
	testSuite.TestCaseCount = len(testSuite.TestCases)

	response.SuccessWithMessage(c, "创建成功", testSuite)
}

func GetTestSuite(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的测试套件ID")
		return
	}

	var testSuite models.TestSuite
	err = database.DB.Preload("Project").Preload("Environment").Preload("User").Preload("TestCases").
		Where("status = ?", 1).First(&testSuite, id).Error
	if err != nil {
		response.NotFound(c, "测试套件不存在")
		return
	}

	testSuite.User.Password = ""
	testSuite.TestCaseCount = len(testSuite.TestCases)
	response.Success(c, testSuite)
}

func UpdateTestSuite(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的测试套件ID")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req struct {
		Name            string `json:"name" binding:"omitempty,min=1,max=200"`
		Description     string `json:"description" binding:"max=1000"`
		EnvironmentID   uint   `json:"environment_id"`
		TestCaseIDs     []uint `json:"test_case_ids"`
		Tags            string `json:"tags" binding:"max=500"`
		Priority        int    `json:"priority" binding:"min=1,max=3"`
		CronExpression  string `json:"cron_expression" binding:"max=100"`
		IsParallel      bool   `json:"is_parallel"`
		TimeoutMinutes  int    `json:"timeout_minutes" binding:"min=1,max=1440"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var testSuite models.TestSuite
	err = database.DB.Where("id = ? AND user_id = ? AND status = ?", id, userID, 1).
		First(&testSuite).Error
	if err != nil {
		response.NotFound(c, "测试套件不存在或无权限")
		return
	}

	// Check name uniqueness if updating
	if req.Name != "" && req.Name != testSuite.Name {
		var existingTestSuite models.TestSuite
		err := database.DB.Where("name = ? AND project_id = ? AND id != ? AND status = ?",
			req.Name, testSuite.ProjectID, id, 1).First(&existingTestSuite).Error
		if err == nil {
			response.BadRequest(c, "测试套件名称在该项目中已存在")
			return
		}
		testSuite.Name = req.Name
	}

	// Update other fields
	if req.Description != "" {
		testSuite.Description = req.Description
	}
	if req.EnvironmentID != 0 {
		// Verify environment exists
		var environment models.Environment
		err := database.DB.Where("id = ? AND status = ?", req.EnvironmentID, 1).First(&environment).Error
		if err != nil {
			response.NotFound(c, "环境不存在")
			return
		}
		testSuite.EnvironmentID = req.EnvironmentID
	}
	if req.Tags != "" {
		testSuite.Tags = req.Tags
	}
	if req.Priority != 0 {
		testSuite.Priority = req.Priority
	}
	if req.CronExpression != "" {
		testSuite.CronExpression = req.CronExpression
	}
	testSuite.IsParallel = req.IsParallel
	if req.TimeoutMinutes != 0 {
		testSuite.TimeoutMinutes = req.TimeoutMinutes
	}

	// Update test cases if provided
	if req.TestCaseIDs != nil {
		var testCases []models.TestCase
		if len(req.TestCaseIDs) > 0 {
			err = database.DB.Where("id IN ? AND project_id = ? AND status = ?", 
				req.TestCaseIDs, testSuite.ProjectID, 1).Find(&testCases).Error
			if err != nil || len(testCases) != len(req.TestCaseIDs) {
				response.BadRequest(c, "部分测试用例不存在或不属于该项目")
				return
			}
		}

		err = database.DB.Model(&testSuite).Association("TestCases").Replace(testCases)
		if err != nil {
			response.InternalServerError(c, "更新测试用例关联失败")
			return
		}
	}

	err = database.DB.Save(&testSuite).Error
	if err != nil {
		response.InternalServerError(c, "更新测试套件失败")
		return
	}

	// Load relations for response
	database.DB.Preload("Project").Preload("Environment").Preload("User").Preload("TestCases").
		First(&testSuite, testSuite.ID)
	testSuite.User.Password = ""
	testSuite.TestCaseCount = len(testSuite.TestCases)

	response.SuccessWithMessage(c, "更新成功", testSuite)
}

func DeleteTestSuite(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的测试套件ID")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var testSuite models.TestSuite
	err = database.DB.Where("id = ? AND user_id = ? AND status = ?", id, userID, 1).
		First(&testSuite).Error
	if err != nil {
		response.NotFound(c, "测试套件不存在或无权限")
		return
	}

	// Remove test case associations first
	err = database.DB.Model(&testSuite).Association("TestCases").Clear()
	if err != nil {
		response.InternalServerError(c, "清除测试用例关联失败")
		return
	}

	// Soft delete
	testSuite.Status = 0
	err = database.DB.Save(&testSuite).Error
	if err != nil {
		response.InternalServerError(c, "删除测试套件失败")
		return
	}

	response.SuccessWithMessage(c, "删除成功", nil)
}

func ExecuteTestSuite(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的测试套件ID")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	// Parse request body for execution options
	var req struct {
		IsVisual bool `json:"is_visual"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// If no body provided, default to visual execution
		req.IsVisual = true
	}

	var testSuite models.TestSuite
	err = database.DB.Preload("Project").Preload("TestCases", "status = ?", 1).
		Where("id = ? AND status = ?", id, 1).First(&testSuite).Error
	if err != nil {
		response.NotFound(c, "测试套件不存在")
		return
	}

	// Check if user has permission to execute
	if testSuite.UserID != userID.(uint) && testSuite.Project.UserID != userID.(uint) {
		response.Forbidden(c, "无权限执行该测试套件")
		return
	}

	if len(testSuite.TestCases) == 0 {
		response.BadRequest(c, "测试套件中没有测试用例")
		return
	}

	// Check if executor is available
	if executor.GlobalExecutor == nil {
		response.InternalServerError(c, "测试执行引擎未初始化")
		return
	}

	runningCount := executor.GlobalExecutor.GetRunningCount()
	if runningCount + len(testSuite.TestCases) > 10 {
		response.BadRequest(c, "当前并发执行数不足以运行整个测试套件，请稍后再试")
		return
	}

	var executions []models.TestExecution

	// Create execution records for all test cases
	for _, testCase := range testSuite.TestCases {
		execution := models.TestExecution{
			TestCaseID:    testCase.ID,
			TestSuiteID:   &testSuite.ID,
			ExecutionType: "test_suite",
			Status:        "pending",
			StartTime:     time.Now(),
			UserID:        userID.(uint),
			ErrorMessage:  "",
			ExecutionLogs: "[]",
			Screenshots:   "[]",
		}

		err = database.DB.Create(&execution).Error
		if err != nil {
			response.InternalServerError(c, "创建执行记录失败")
			return
		}

		executions = append(executions, execution)
	}

	// Execute all test cases asynchronously
	go func() {
		for i, execution := range executions {
			execution.Status = "running"
			database.DB.Save(&execution)

			// Load test case with relations
			var testCase models.TestCase
			database.DB.Preload("Environment").Preload("Device").
				First(&testCase, execution.TestCaseID)

			resultChan := executor.GlobalExecutor.ExecuteTestCaseWithOptions(&execution, &testCase, req.IsVisual)
			result := <-resultChan

			// Update execution with result
			if result.Success {
				execution.Status = "passed"
			} else {
				execution.Status = "failed"
				execution.ErrorMessage = result.ErrorMessage
			}

			now := time.Now()
			execution.EndTime = &now
			execution.Duration = int(now.Sub(execution.StartTime).Seconds())

			// Save logs and screenshots
			if logsJSON, err := json.Marshal(result.Logs); err == nil {
				execution.ExecutionLogs = string(logsJSON)
			}
			if screenshotsJSON, err := json.Marshal(result.Screenshots); err == nil {
				execution.Screenshots = string(screenshotsJSON)
			}

			database.DB.Save(&execution)

			// Save performance metrics if available
			if result.Metrics != nil {
				result.Metrics.ExecutionID = execution.ID
				database.DB.Create(result.Metrics)
			}

			executions[i] = execution
		}
	}()

	// Load executions with relations for response
	for i := range executions {
		database.DB.Preload("TestCase").Preload("User").First(&executions[i], executions[i].ID)
		executions[i].User.Password = ""
	}

	response.SuccessWithMessage(c, "测试套件执行已启动", executions)
}

func StopTestSuite(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的测试套件ID")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var testSuite models.TestSuite
	err = database.DB.Where("id = ? AND status = ?", id, 1).First(&testSuite).Error
	if err != nil {
		response.NotFound(c, "测试套件不存在")
		return
	}

	// Check if user has permission to stop
	if testSuite.UserID != userID.(uint) {
		response.Forbidden(c, "无权限停止该测试套件")
		return
	}

	// Find all running or pending executions for this test suite
	var executions []models.TestExecution
	err = database.DB.Where("test_suite_id = ? AND (status = ? OR status = ?)", 
		id, "running", "pending").Find(&executions).Error
	if err != nil {
		response.InternalServerError(c, "查询执行记录失败")
		return
	}

	if len(executions) == 0 {
		response.BadRequest(c, "没有正在运行的执行记录")
		return
	}

	// Stop all running/pending executions
	err = database.DB.Model(&models.TestExecution{}).
		Where("test_suite_id = ? AND (status = ? OR status = ?)", id, "running", "pending").
		Updates(models.TestExecution{Status: "cancelled"}).Error
	if err != nil {
		response.InternalServerError(c, "停止测试套件执行失败")
		return
	}

	response.SuccessWithMessage(c, "测试套件执行已停止", gin.H{
		"stopped_count": len(executions),
	})
}
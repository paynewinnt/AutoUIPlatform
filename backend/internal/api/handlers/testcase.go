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

func GetTestCases(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	projectID := c.Query("project_id")

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}

	var testCases []models.TestCase
	var total int64

	query := database.DB.Model(&models.TestCase{}).Where("status = ?", 1)
	if projectID != "" {
		query = query.Where("project_id = ?", projectID)
	}

	// Count total
	query.Count(&total)

	// Get paginated test cases with relations
	offset := (page - 1) * pageSize
	err := query.Preload("Project").Preload("Environment").Preload("Device").Preload("User").
		Offset(offset).Limit(pageSize).Find(&testCases).Error
	if err != nil {
		response.InternalServerError(c, "获取测试用例列表失败")
		return
	}

	// Clear user passwords
	for i := range testCases {
		testCases[i].User.Password = ""
	}

	response.Page(c, testCases, total, page, pageSize)
}

func CreateTestCase(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req struct {
		Name           string                `json:"name" binding:"required,min=1,max=200"`
		Description    string                `json:"description" binding:"max=1000"`
		ProjectID      uint                  `json:"project_id" binding:"required"`
		EnvironmentID  uint                  `json:"environment_id" binding:"required"`
		DeviceID       uint                  `json:"device_id" binding:"required"`
		TargetURL      string                `json:"target_url" binding:"required,url"`
		Steps          []models.TestStep     `json:"steps"`
		ExpectedResult string                `json:"expected_result" binding:"max=1000"`
		Tags           string                `json:"tags" binding:"max=500"`
		Priority       int                   `json:"priority" binding:"min=1,max=3"`
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

	// Verify device exists
	var device models.Device
	err = database.DB.Where("id = ? AND status = ?", req.DeviceID, 1).First(&device).Error
	if err != nil {
		response.NotFound(c, "设备不存在")
		return
	}

	// Convert steps to JSON
	stepsJSON := "[]"
	if len(req.Steps) > 0 {
		if data, err := json.Marshal(req.Steps); err == nil {
			stepsJSON = string(data)
		}
	}

	// Check if test case name exists in the project
	var existingTestCase models.TestCase
	err = database.DB.Where("name = ? AND project_id = ? AND status = ?", req.Name, req.ProjectID, 1).
		First(&existingTestCase).Error
	if err == nil {
		response.BadRequest(c, "测试用例名称在该项目中已存在")
		return
	}

	testCase := models.TestCase{
		Name:           req.Name,
		Description:    req.Description,
		ProjectID:      req.ProjectID,
		EnvironmentID:  req.EnvironmentID,
		DeviceID:       req.DeviceID,
		TargetURL:      req.TargetURL,
		Steps:          stepsJSON,
		ExpectedResult: req.ExpectedResult,
		Tags:           req.Tags,
		Priority:       req.Priority,
		Status:         1,
		UserID:         userID.(uint),
	}

	err = database.DB.Create(&testCase).Error
	if err != nil {
		response.InternalServerError(c, "创建测试用例失败")
		return
	}

	// Load relations for response
	database.DB.Preload("Project").Preload("Environment").Preload("Device").Preload("User").
		First(&testCase, testCase.ID)
	testCase.User.Password = ""

	response.SuccessWithMessage(c, "创建成功", testCase)
}

func GetTestCase(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的测试用例ID")
		return
	}

	var testCase models.TestCase
	err = database.DB.Preload("Project").Preload("Environment").Preload("Device").Preload("User").
		Where("status = ?", 1).First(&testCase, id).Error
	if err != nil {
		response.NotFound(c, "测试用例不存在")
		return
	}

	testCase.User.Password = ""
	response.Success(c, testCase)
}

func UpdateTestCase(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的测试用例ID")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req struct {
		Name           string            `json:"name" binding:"omitempty,min=1,max=200"`
		Description    string            `json:"description" binding:"max=1000"`
		ProjectID      uint              `json:"project_id"`
		EnvironmentID  uint              `json:"environment_id"`
		DeviceID       uint              `json:"device_id"`
		TargetURL      string            `json:"target_url" binding:"omitempty,url"`
		Steps          []models.TestStep `json:"steps"`
		ExpectedResult string            `json:"expected_result" binding:"max=1000"`
		Tags           string            `json:"tags" binding:"max=500"`
		Priority       int               `json:"priority" binding:"omitempty,min=1,max=3"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var testCase models.TestCase
	err = database.DB.Where("id = ? AND user_id = ? AND status = ?", id, userID, 1).
		First(&testCase).Error
	if err != nil {
		response.NotFound(c, "测试用例不存在或无权限")
		return
	}

	// Verify project if updating
	if req.ProjectID > 0 && req.ProjectID != testCase.ProjectID {
		var project models.Project
		err := database.DB.Where("id = ? AND user_id = ? AND status = ?", req.ProjectID, userID, 1).
			First(&project).Error
		if err != nil {
			response.NotFound(c, "项目不存在或无权限")
			return
		}
		testCase.ProjectID = req.ProjectID
	}

	// Verify environment if updating
	if req.EnvironmentID > 0 && req.EnvironmentID != testCase.EnvironmentID {
		var environment models.Environment
		err := database.DB.Where("id = ? AND status = ?", req.EnvironmentID, 1).First(&environment).Error
		if err != nil {
			response.NotFound(c, "环境不存在")
			return
		}
		testCase.EnvironmentID = req.EnvironmentID
	}

	// Verify device if updating
	if req.DeviceID > 0 && req.DeviceID != testCase.DeviceID {
		var device models.Device
		err := database.DB.Where("id = ? AND status = ?", req.DeviceID, 1).First(&device).Error
		if err != nil {
			response.NotFound(c, "设备不存在")
			return
		}
		testCase.DeviceID = req.DeviceID
	}

	// Check name uniqueness if updating
	if req.Name != "" && req.Name != testCase.Name {
		var existingTestCase models.TestCase
		err := database.DB.Where("name = ? AND project_id = ? AND id != ? AND status = ?",
			req.Name, testCase.ProjectID, id, 1).First(&existingTestCase).Error
		if err == nil {
			response.BadRequest(c, "测试用例名称在该项目中已存在")
			return
		}
		testCase.Name = req.Name
	}

	// Update other fields
	if req.Description != "" {
		testCase.Description = req.Description
	}
	if req.TargetURL != "" {
		testCase.TargetURL = req.TargetURL
	}
	if req.ExpectedResult != "" {
		testCase.ExpectedResult = req.ExpectedResult
	}
	if req.Tags != "" {
		testCase.Tags = req.Tags
	}
	if req.Priority > 0 {
		testCase.Priority = req.Priority
	}

	// Update steps if provided
	if req.Steps != nil {
		if data, err := json.Marshal(req.Steps); err == nil {
			testCase.Steps = string(data)
		}
	}

	err = database.DB.Save(&testCase).Error
	if err != nil {
		response.InternalServerError(c, "更新测试用例失败")
		return
	}

	// Load relations for response
	database.DB.Preload("Project").Preload("Environment").Preload("Device").Preload("User").
		First(&testCase, testCase.ID)
	testCase.User.Password = ""

	response.SuccessWithMessage(c, "更新成功", testCase)
}

func DeleteTestCase(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的测试用例ID")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var testCase models.TestCase
	err = database.DB.Where("id = ? AND user_id = ? AND status = ?", id, userID, 1).
		First(&testCase).Error
	if err != nil {
		response.NotFound(c, "测试用例不存在或无权限")
		return
	}

	// Check if test case is in any test suite
	var testSuiteCount int64
	database.DB.Table("test_suite_cases").Where("test_case_id = ?", id).Count(&testSuiteCount)
	if testSuiteCount > 0 {
		response.BadRequest(c, "该测试用例正在被测试套件使用，无法删除")
		return
	}

	// Soft delete
	testCase.Status = 0
	err = database.DB.Save(&testCase).Error
	if err != nil {
		response.InternalServerError(c, "删除测试用例失败")
		return
	}

	response.SuccessWithMessage(c, "删除成功", nil)
}

func ExecuteTestCase(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的测试用例ID")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var testCase models.TestCase
	err = database.DB.Preload("Project").Preload("Environment").Preload("Device").
		Where("id = ? AND status = ?", id, 1).First(&testCase).Error
	if err != nil {
		response.NotFound(c, "测试用例不存在")
		return
	}

	// Check if user has permission to execute (same user or project owner)
	if testCase.UserID != userID.(uint) && testCase.Project.UserID != userID.(uint) {
		response.Forbidden(c, "无权限执行该测试用例")
		return
	}

	// Check if executor is available
	if executor.GlobalExecutor == nil {
		response.InternalServerError(c, "测试执行引擎未初始化")
		return
	}

	runningCount := executor.GlobalExecutor.GetRunningCount()
	if runningCount >= 10 { // Max concurrent executions
		response.BadRequest(c, "当前并发执行数已达上限，请稍后再试")
		return
	}

	// Create execution record
	execution := models.TestExecution{
		TestCaseID:    testCase.ID,
		ExecutionType: "test_case",
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

	// Update status to running
	execution.Status = "running"
	database.DB.Save(&execution)

	// Execute test case asynchronously (default to visual execution)
	go func() {
		resultChan := executor.GlobalExecutor.ExecuteTestCaseWithOptions(&execution, &testCase, true)
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
	}()

	// Load execution with relations for response
	database.DB.Preload("TestCase").Preload("User").First(&execution, execution.ID)
	execution.User.Password = ""

	response.SuccessWithMessage(c, "测试执行已启动", execution)
}
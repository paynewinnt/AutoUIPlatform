package handlers

import (
	"autoui-platform/backend/internal/models"
	"autoui-platform/backend/pkg/database"
	"autoui-platform/backend/pkg/response"
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetEnvironments(c *gin.Context) {
	var environments []models.Environment
	err := database.DB.Where("status = ?", 1).Find(&environments).Error
	if err != nil {
		response.InternalServerError(c, "获取环境列表失败")
		return
	}

	response.Success(c, environments)
}

func CreateEnvironment(c *gin.Context) {
	var req struct {
		Name        string                 `json:"name" binding:"required,min=1,max=100"`
		Description string                 `json:"description" binding:"max=500"`
		BaseURL     string                 `json:"base_url" binding:"required,url"`
		Type        string                 `json:"type" binding:"required,oneof=test product"`
		Headers     map[string]interface{} `json:"headers"`
		Variables   map[string]interface{} `json:"variables"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Check if environment name and type combination exists
	var existingEnv models.Environment
	err := database.DB.Where("name = ? AND type = ? AND status = ?", req.Name, req.Type, 1).
		First(&existingEnv).Error
	if err == nil {
		response.BadRequest(c, "相同类型的环境名称已存在")
		return
	}

	// Convert maps to JSON strings
	headersJSON := "{}"
	if req.Headers != nil {
		if data, err := json.Marshal(req.Headers); err == nil {
			headersJSON = string(data)
		}
	}

	variablesJSON := "{}"
	if req.Variables != nil {
		if data, err := json.Marshal(req.Variables); err == nil {
			variablesJSON = string(data)
		}
	}

	environment := models.Environment{
		Name:        req.Name,
		Description: req.Description,
		BaseURL:     req.BaseURL,
		Type:        req.Type,
		Headers:     headersJSON,
		Variables:   variablesJSON,
		Status:      1,
	}

	err = database.DB.Create(&environment).Error
	if err != nil {
		response.InternalServerError(c, "创建环境失败")
		return
	}

	response.SuccessWithMessage(c, "创建成功", environment)
}

func GetEnvironment(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的环境ID")
		return
	}

	var environment models.Environment
	err = database.DB.Where("status = ?", 1).First(&environment, id).Error
	if err != nil {
		response.NotFound(c, "环境不存在")
		return
	}

	response.Success(c, environment)
}

func UpdateEnvironment(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的环境ID")
		return
	}

	var req struct {
		Name        string                 `json:"name" binding:"omitempty,min=1,max=100"`
		Description string                 `json:"description" binding:"max=500"`
		BaseURL     string                 `json:"base_url" binding:"omitempty,url"`
		Type        string                 `json:"type" binding:"omitempty,oneof=test product"`
		Headers     map[string]interface{} `json:"headers"`
		Variables   map[string]interface{} `json:"variables"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var environment models.Environment
	err = database.DB.Where("status = ?", 1).First(&environment, id).Error
	if err != nil {
		response.NotFound(c, "环境不存在")
		return
	}

	// Check name and type uniqueness if updating
	if req.Name != "" && req.Type != "" && (req.Name != environment.Name || req.Type != environment.Type) {
		var existingEnv models.Environment
		err := database.DB.Where("name = ? AND type = ? AND id != ? AND status = ?", 
			req.Name, req.Type, id, 1).First(&existingEnv).Error
		if err == nil {
			response.BadRequest(c, "相同类型的环境名称已存在")
			return
		}
	}

	// Update fields
	if req.Name != "" {
		environment.Name = req.Name
	}
	if req.Description != "" {
		environment.Description = req.Description
	}
	if req.BaseURL != "" {
		environment.BaseURL = req.BaseURL
	}
	if req.Type != "" {
		environment.Type = req.Type
	}

	// Update headers if provided
	if req.Headers != nil {
		if data, err := json.Marshal(req.Headers); err == nil {
			environment.Headers = string(data)
		}
	}

	// Update variables if provided
	if req.Variables != nil {
		if data, err := json.Marshal(req.Variables); err == nil {
			environment.Variables = string(data)
		}
	}

	err = database.DB.Save(&environment).Error
	if err != nil {
		response.InternalServerError(c, "更新环境失败")
		return
	}

	response.SuccessWithMessage(c, "更新成功", environment)
}

func DeleteEnvironment(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "无效的环境ID")
		return
	}

	var environment models.Environment
	err = database.DB.Where("status = ?", 1).First(&environment, id).Error
	if err != nil {
		response.NotFound(c, "环境不存在")
		return
	}

	// Check if environment is being used by test cases
	var testCaseCount int64
	database.DB.Model(&models.TestCase{}).Where("environment_id = ? AND status = ?", id, 1).Count(&testCaseCount)
	if testCaseCount > 0 {
		response.BadRequest(c, "该环境正在被测试用例使用，无法删除")
		return
	}

	// Soft delete
	environment.Status = 0
	err = database.DB.Save(&environment).Error
	if err != nil {
		response.InternalServerError(c, "删除环境失败")
		return
	}

	response.SuccessWithMessage(c, "删除成功", nil)
}
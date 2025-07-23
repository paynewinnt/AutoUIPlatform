package handlers

import (
	"autoui-platform/backend/internal/models"
	"autoui-platform/backend/pkg/database"
	"autoui-platform/backend/pkg/response"
	"autoui-platform/backend/pkg/utils"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var user models.User
	err := database.DB.First(&user, userID).Error
	if err != nil {
		response.InternalServerError(c, "获取用户信息失败")
		return
	}

	// Clear password
	user.Password = ""
	response.Success(c, user)
}

func UpdateProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "用户未登录")
		return
	}

	var req struct {
		Username string `json:"username" binding:"omitempty,min=3"`
		Email    string `json:"email" binding:"omitempty,email"`
		Avatar   string `json:"avatar"`
		Password string `json:"password" binding:"omitempty,min=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var user models.User
	err := database.DB.First(&user, userID).Error
	if err != nil {
		response.InternalServerError(c, "获取用户信息失败")
		return
	}

	// Check username uniqueness if updating
	if req.Username != "" && req.Username != user.Username {
		var existingUser models.User
		err := database.DB.Where("username = ? AND id != ?", req.Username, userID).First(&existingUser).Error
		if err == nil {
			response.BadRequest(c, "用户名已存在")
			return
		}
		user.Username = req.Username
	}

	// Check email uniqueness if updating
	if req.Email != "" && req.Email != user.Email {
		var existingUser models.User
		err := database.DB.Where("email = ? AND id != ?", req.Email, userID).First(&existingUser).Error
		if err == nil {
			response.BadRequest(c, "邮箱已被使用")
			return
		}
		user.Email = req.Email
	}

	// Update avatar if provided
	if req.Avatar != "" {
		user.Avatar = req.Avatar
	}

	// Update password if provided
	if req.Password != "" {
		hashedPassword, err := utils.HashPassword(req.Password)
		if err != nil {
			response.InternalServerError(c, "密码加密失败")
			return
		}
		user.Password = hashedPassword
	}

	err = database.DB.Save(&user).Error
	if err != nil {
		response.InternalServerError(c, "更新用户信息失败")
		return
	}

	// Clear password from response
	user.Password = ""
	response.SuccessWithMessage(c, "更新成功", user)
}

func GetUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}

	var users []models.User
	var total int64

	// Count total
	database.DB.Model(&models.User{}).Count(&total)

	// Get paginated users
	offset := (page - 1) * pageSize
	err := database.DB.Select("id, username, email, avatar, status, created_at, updated_at").
		Offset(offset).Limit(pageSize).Find(&users).Error
	if err != nil {
		response.InternalServerError(c, "获取用户列表失败")
		return
	}

	response.Page(c, users, total, page, pageSize)
}
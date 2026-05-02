package login

import (
	"ffmpegserver/model"
	"ffmpegserver/public/sql"
	"ffmpegserver/types/login"
	"ffmpegserver/utils"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Handler 登录处理器
type Handler struct{}

// NewHandler 创建登录处理器
func NewHandler() *Handler {
	return &Handler{}
}

// Register 注册路由
func (h *Handler) Register(r *gin.RouterGroup) {
	r.POST("/login", h.Login)
	r.POST("/refresh", h.RefreshToken)
	r.POST("/check", h.CheckLogin)
}

// Login
// @Summary 用户登录（账号不存在时自动创建）
// @Description 使用账号密码登录。如果账号不存在，会自动创建新用户，然后返回双 Token
// @Tags 认证 - AUTH
// @Accept json
// @Produce json
// @Param body body login.PostLogin true "登录信息"
// @Success 200 {object} login.LoginResponse
// @Failure 400 {object} map[string]string
// @Router /api/auth/login [POST]
func (h *Handler) Login(c *gin.Context) {
	var form login.PostLogin
	if err := c.ShouldBindJSON(&form); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	if form.Account == "" || form.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "账号和密码不能为空"})
		return
	}

	// 查询用户
	var user model.User
	err := sql.Gdb.Where("account = ?", form.Account).First(&user).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 账号不存在 → 自动创建
			hashedPwd, hashErr := utils.HashPassword(form.Password)
			if hashErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
				return
			}

			now := time.Now().Unix()
			user = model.User{
				Account:      form.Account,
				Password:     hashedPwd,
				NickName:     randomNickname(),
				Avatar:       randomAvatar(),
				Role:         0,
				LoginIP:      c.ClientIP(),
				LoginTime:    now,
				CreationTime: now,
				UpdateTime:   now,
			}

			if createErr := sql.Gdb.Create(&user).Error; createErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "用户创建失败"})
				return
			}
			fmt.Printf("[登录] 新用户已自动创建: account=%s, id=%d\n", user.Account, user.ID)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户失败"})
			return
		}
	} else {
		// 账号已存在 → 验证密码
		if !utils.CheckPassword(form.Password, user.Password) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
			return
		}

		// 更新登录信息
		now := time.Now().Unix()
		sql.Gdb.Model(&user).Updates(map[string]interface{}{
			"login_ip":    c.ClientIP(),
			"login_time":  now,
			"update_time": now,
		})
		user.LoginIP = c.ClientIP()
		user.LoginTime = time.Now().Unix()
	}

	// 生成双 Token
	accessToken, aErr := utils.GenerateAccessToken(user.ID)
	if aErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AccessToken 生成失败"})
		return
	}

	refreshToken, rErr := utils.GenerateRefreshToken(user.ID)
	if rErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "RefreshToken 生成失败"})
		return
	}

	// 计算过期时间戳
	expiresIn := time.Now().Add(time.Duration(15) * time.Minute).Unix()

	c.JSON(http.StatusOK, login.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
		User: login.UserInfo{
			ID:       user.ID,
			Account:  user.Account,
			NickName: user.NickName,
			Avatar:   user.Avatar,
			Role:     user.Role,
		},
	})
}

// RefreshToken
// @Summary 刷新 Access Token
// @Description 使用 Refresh Token 获取新的 Access Token
// @Tags 认证 - AUTH
// @Accept json
// @Produce json
// @Param body body login.PostRefreshToken true "RefreshToken"
// @Success 200 {object} login.RefreshTokenResponse
// @Failure 400 {object} map[string]string
// @Router /api/auth/refresh [POST]
func (h *Handler) RefreshToken(c *gin.Context) {
	var form login.PostRefreshToken
	if err := c.ShouldBindJSON(&form); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 解析 Refresh Token
	claims, err := utils.ParseToken(form.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "RefreshToken 无效或已过期"})
		return
	}

	// 校验 Token 类型
	if claims.TokenType != "refresh" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token 类型错误，请使用 Refresh Token"})
		return
	}

	// 验证用户是否存在
	var user model.User
	if err := sql.Gdb.Where("id = ?", claims.UserID).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}

	// 生成新的 Access Token
	accessToken, aErr := utils.GenerateAccessToken(user.ID)
	if aErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AccessToken 生成失败"})
		return
	}

	expiresIn := time.Now().Add(time.Duration(15) * time.Minute).Unix()

	c.JSON(http.StatusOK, login.RefreshTokenResponse{
		AccessToken: accessToken,
		ExpiresIn:   expiresIn,
	})
}

// CheckLogin
// @Summary 校验登录状态
// @Description 验证 Access Token 是否有效，并返回用户信息
// @Tags 认证 - AUTH
// @Produce json
// @Success 200 {object} login.CheckLoginResponse
// @Failure 401 {object} map[string]string
// @Router /api/auth/check [POST]
func (h *Handler) CheckLogin(c *gin.Context) {
	// 从 JWT 中间件注入的上下文取 user_id
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusOK, login.CheckLoginResponse{Valid: false})
		return
	}

	id, ok := userID.(int32)
	if !ok {
		c.JSON(http.StatusOK, login.CheckLoginResponse{Valid: false})
		return
	}

	// 查询用户信息
	var user model.User
	if err := sql.Gdb.Where("id = ?", id).First(&user).Error; err != nil {
		c.JSON(http.StatusOK, login.CheckLoginResponse{Valid: false})
		return
	}

	c.JSON(http.StatusOK, login.CheckLoginResponse{
		Valid: true,
		User: login.UserInfo{
			ID:       user.ID,
			Account:  user.Account,
			NickName: user.NickName,
			Avatar:   user.Avatar,
			Role:     user.Role,
		},
	})
}

// randomNickname 生成随机昵称
func randomNickname() string {
	prefixes := []string{"湫创", "灵境", "AI", "Chiu", "PC"}
	suffix := rand.Intn(99999)
	return fmt.Sprintf("%s_%d", prefixes[rand.Intn(len(prefixes))], suffix)
}

// randomAvatar 随机返回头像
func randomAvatar() string {
	return fmt.Sprintf("/avatar/%d.png", rand.Intn(10)+1)
}

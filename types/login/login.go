package login

// PostLogin 登录请求
type PostLogin struct {
	Account    string `json:"account" binding:"required" example:"admin"`   // 账号
	Password   string `json:"password" binding:"required" example:"123456"` // 密码
	PCCode     string `json:"pc_code"`                                      // 设备唯一码（登录时自动注册）
	DeviceName string `json:"device_name"`                                  // 设备名称（计算机名）
}

// LoginResponse 登录成功响应
type LoginResponse struct {
	AccessToken  string   `json:"access_token"`  // AccessToken
	RefreshToken string   `json:"refresh_token"` // RefreshToken
	ExpiresIn    int64    `json:"expires_in"`    // AccessToken 过期时间戳
	User         UserInfo `json:"user"`          // 用户信息
}

// UserInfo 对外暴露的用户信息
type UserInfo struct {
	ID       int32  `json:"id"`
	Account  string `json:"account"`
	NickName string `json:"nick_name"`
	Avatar   string `json:"avatar"`
	Role     int32  `json:"role"`
}

// PostRefreshToken RefreshToken 请求
type PostRefreshToken struct {
	RefreshToken string `json:"refresh_token" binding:"required"` // RefreshToken
}

// RefreshTokenResponse RefreshToken 响应
type RefreshTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

// PostCheckLogin 校验登录请求
type PostCheckLogin struct {
	AccessToken string `json:"access_token" binding:"required"` // AccessToken
}

// CheckLoginResponse 校验登录响应
type CheckLoginResponse struct {
	Valid bool     `json:"valid"` // Token 是否有效
	User  UserInfo `json:"user"`  // 用户信息
}

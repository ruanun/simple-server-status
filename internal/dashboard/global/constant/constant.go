package constant

// HeaderSecret 定义认证密钥的 HTTP 头名称（这是头名称，不是实际凭证）
//
//nolint:gosec // G101: 这是 HTTP 头名称常量，不是硬编码的凭证值
const HeaderSecret = "X-AUTH-SECRET"

// HeaderId 定义服务器ID的 HTTP 头名称
const HeaderId = "X-SERVER-ID"

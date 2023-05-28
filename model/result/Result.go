package result

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

var (
	OK              = Error(200, "success")
	NeedRedirect    = Error(301, "need redirect")
	InvalidArgs     = Error(400, "invalid params")
	Unauthorized    = Error(401, "unauthorized")
	Forbidden       = Error(403, "forbidden")
	NotFound        = Error(404, "not found")
	Conflict        = Error(409, "entry exist")
	TooManyRequests = Error(429, "too many requests")
	ResultError     = Error(500, "response error")
	DatabaseError   = Error(598, "database error")
	CSRFDetected    = Error(599, "csrf attack detected")

	UserError  = Error(5001, "username or password error")
	CodeExpire = Error(5002, "verification expire")
	CodeError  = Error(5003, "verification error")
	UserExist  = Error(5004, "user Exist")
)

type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func Result(c *gin.Context, code int, msg string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		code,
		msg,
		data,
	})
}
func Error(code int, msg string) Response {
	return Response{
		code,
		msg,
		nil,
	}
}

func Ok(c *gin.Context) {
	OkWithData(c, nil)
}

func OkWithData(c *gin.Context, data interface{}) {
	Result(c, OK.Code, OK.Msg, data)
}

func OkWithMsg(c *gin.Context, msg string) {
	Result(c, OK.Code, msg, nil)
}

func Fail(c *gin.Context, err Response) {
	Result(c, err.Code, err.Msg, nil)
}

func FailWithMsg(c *gin.Context, err Response, msg string) {
	Result(c, err.Code, msg, nil)
}

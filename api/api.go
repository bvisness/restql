package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ErrorOutput struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Response struct {
	Data   interface{}   `json:"data,omitempty"`
	Errors []ErrorOutput `json:"errors,omitempty"`
}

type ErrorWithRestStatus struct {
	error
	Status int
}

func NewErrorWithRestStatus(status int, err error) ErrorWithRestStatus {
	return ErrorWithRestStatus{
		error:  err,
		Status: status,
	}
}

func (err ErrorWithRestStatus) GetWrappedError() error {
	return err.error
}

type RestResponseErrorOutput struct {
	Message string `json:"message"`
}

func ErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if !c.IsAborted() {
			return
		}

		if len(c.Errors) == 0 {
			return
		}

		for _, err := range c.Errors {
			errOutput := RestResponseErrorOutput{
				Message: err.Error(),
			}

			status := http.StatusInternalServerError
			if restErr, isRestErr := err.Err.(ErrorWithRestStatus); isRestErr {
				status = restErr.Status
			}

			c.JSON(status, errOutput)
		}
	}
}

package main

//=============================================================================
import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/kardianos/service"
)

//=============================================================================
type program struct {
}

func (prg *program) Start(s service.Service) (err error) {
	fmt.Println("================Start================")
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	r.Run()
	return nil
}

func (prg *program) Stop(s service.Service) (err error) {
	fmt.Println("================Stop================")
	return nil
}

//=============================================================================
func main() {
	cfg := &service.Config{
		Name:        "IVS-SMS",
		DisplayName: "IVS-SMS",
		Description: "IVS-SMS",
	}
	s, err := service.New(new(program), cfg)
	if err == nil {
		s.Run()
	}
}

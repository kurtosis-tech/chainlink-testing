package main

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"net/http"
)

const (
	httpPort = "1323"
	sampleUsdResponse = 1675.58
)

type PriceFeedResponse struct {
	Usd float32 `json:"USD"`
}

func main() {
	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.GET("/", standardJsonResponse)

	// Start server
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%v", httpPort)))
}

// Handler
func standardJsonResponse(c echo.Context) error {
	response := PriceFeedResponse{
		Usd: sampleUsdResponse,
	}
	return c.JSON(http.StatusOK, response)
}
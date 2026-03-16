package httpserver

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func (s *server) routes() *fiber.App {
	app := fiber.New(fiber.Config{AppName: "ZeroNime API v2"})
	app.Use(cors.New(cors.Config{
		AllowHeaders: "Origin, Content-Type, Accept, X-Client-Token",
		AllowMethods: "GET,POST,DELETE,OPTIONS",
		AllowOrigins: "*",
	}))
	app.Use(s.requestLogger)
	app.Use(s.rateLimit)

	app.Get("/health", s.health)

	v2 := app.Group("/api/v2")
	v2.Post("/session/anonymous", s.ensureSession)
	v2.Get("/home", s.home)
	v2.Get("/search", s.search)
	v2.Get("/schedule", s.schedule)
	v2.Get("/index", s.index)
	v2.Get("/discover/seasonal-popular", s.seasonalPopular)
	v2.Get("/discover/properties/:kind", s.propertyList)
	v2.Get("/discover/properties/:kind/:propertyId", s.propertyCatalog)
	v2.Get("/discover/quick/:kind", s.quickCatalog)
	v2.Get("/discover/schedule", s.schedulePage)
	v2.Get("/catalog/:catalogId", s.catalogDetail)
	v2.Get("/catalog/:catalogId/episodes", s.catalogEpisodes)
	v2.Get("/stream/:episodeId", s.streamResolve)
	v2.Get("/image", s.imageRoute)
	v2.Get("/media", s.mediaRoute)

	protected := v2.Group("/", s.requireClient)
	protected.Get("/watchlist", s.watchlist)
	protected.Post("/watchlist", s.saveWatchlist)
	protected.Delete("/watchlist/:catalogId", s.deleteWatchlist)
	protected.Get("/history", s.history)
	protected.Post("/history", s.saveHistory)
	protected.Delete("/history/:catalogId", s.deleteHistory)
	protected.Get("/continue-watching", s.continueWatching)
	protected.Post("/stream/window", s.primeStartupWindow)
	return app
}

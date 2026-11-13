package deployhandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/deploy"
)

// BuildSchema serves the published build-recipe JSON Schema, which the build CLI
// validates a recipe against. Like the manifest schema it is public: identical
// for every tenant, so it needs no caller.
// @Summary      The build-recipe JSON Schema
// @Description  The versioned JSON Schema for vyzorix.build.toml. Generated from the vyzorix-spec module.
// @Tags         deploy
// @Produce      json
// @Param        version  path  string  true  "schema version"
// @Success      200  {object}  map[string]any  "the JSON Schema"
// @Failure      404  {object}  openapi.ErrorResponse  "unknown schema version"
// @Router       /deploy/build-schema/{version} [get]
func (h *Handler) BuildSchema(c *gin.Context) {
	schema, err := deploy.BuildRecipeSchema(c.Param("version"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})

		return
	}

	c.Data(http.StatusOK, "application/schema+json; charset=utf-8", schema)
}

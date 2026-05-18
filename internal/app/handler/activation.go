package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"activation-service/internal/domain/model"
	"activation-service/internal/domain/ports/request"
)

// ActivationGinHandler holds all 4 domain use cases and exposes HTTP handlers.
type ActivationGinHandler struct {
	greedyBaseline  request.ActivationUseCase
	greedyDB        request.ActivationUseCase
	knapsackMemory  request.ActivationUseCase
	knapsackDB      request.ActivationUseCase
}

// NewActivationGinHandler constructs the handler with injected use cases.
func NewActivationGinHandler(
	greedyBaseline,
	greedyDB,
	knapsackMemory,
	knapsackDB request.ActivationUseCase,
) *ActivationGinHandler {
	return &ActivationGinHandler{
		greedyBaseline: greedyBaseline,
		greedyDB:       greedyDB,
		knapsackMemory: knapsackMemory,
		knapsackDB:     knapsackDB,
	}
}

func handle(c *gin.Context, uc request.ActivationUseCase) {
	var req model.ActivationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := uc.Execute(req)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// HandleGreedyBaseline godoc
// @Summary     Activate assets using greedy baseline (in-memory sort)
// @Tags        activation
// @Accept      json
// @Produce     json
// @Param       request body model.ActivationRequest true "Activation request"
// @Success     200 {object} model.AllocationResult
// @Failure     400 {object} map[string]string
// @Failure     422 {object} map[string]string
// @Router      /api/v1/activation/greedy-baseline [post]
func (h *ActivationGinHandler) HandleGreedyBaseline(c *gin.Context) {
	handle(c, h.greedyBaseline)
}

// HandleGreedyDB godoc
// @Summary     Activate assets using greedy DB (DB-sorted)
// @Tags        activation
// @Accept      json
// @Produce     json
// @Param       request body model.ActivationRequest true "Activation request"
// @Success     200 {object} model.AllocationResult
// @Failure     400 {object} map[string]string
// @Failure     422 {object} map[string]string
// @Router      /api/v1/activation/greedy-db [post]
func (h *ActivationGinHandler) HandleGreedyDB(c *gin.Context) {
	handle(c, h.greedyDB)
}

// HandleKnapsackMemory godoc
// @Summary     Activate assets using knapsack DP (in-memory)
// @Tags        activation
// @Accept      json
// @Produce     json
// @Param       request body model.ActivationRequest true "Activation request"
// @Success     200 {object} model.AllocationResult
// @Failure     400 {object} map[string]string
// @Failure     422 {object} map[string]string
// @Router      /api/v1/activation/knapsack-memory [post]
func (h *ActivationGinHandler) HandleKnapsackMemory(c *gin.Context) {
	handle(c, h.knapsackMemory)
}

// HandleKnapsackDB godoc
// @Summary     Activate assets using knapsack DP (DB-pruned pool)
// @Tags        activation
// @Accept      json
// @Produce     json
// @Param       request body model.ActivationRequest true "Activation request"
// @Success     200 {object} model.AllocationResult
// @Failure     400 {object} map[string]string
// @Failure     422 {object} map[string]string
// @Router      /api/v1/activation/knapsack-db [post]
func (h *ActivationGinHandler) HandleKnapsackDB(c *gin.Context) {
	handle(c, h.knapsackDB)
}

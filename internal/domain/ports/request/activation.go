package request

import "activation-service/internal/domain/model"

// ActivationUseCase is the inbound port for asset activation operations.
type ActivationUseCase interface {
	Execute(req model.ActivationRequest) (model.AllocationResult, error)
}

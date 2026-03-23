package service

import (
	"context"
	"net/http"

	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/manager/errs"
	"github.com/Gthulhu/api/pkg/logger"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (svc *Service) CreatePodSchedulingMetrics(ctx context.Context, operator *domain.Claims, psm *domain.PodSchedulingMetrics) error {
	operatorID, err := operator.GetBsonObjectUID()
	if err != nil {
		return errors.WithMessagef(err, "invalid operator ID %s", operator.UID)
	}

	psm.BaseEntity = domain.NewBaseEntity(&operatorID, &operatorID)
	if psm.CollectionIntervalSeconds == 0 {
		psm.CollectionIntervalSeconds = 10
	}

	if err := svc.Repo.CreatePSM(ctx, psm); err != nil {
		return err
	}

	logger.Logger(ctx).Info().Msgf("created PodSchedulingMetrics %s", psm.ID.Hex())
	return nil
}

func (svc *Service) ListPodSchedulingMetrics(ctx context.Context, opt *domain.QueryPSMOptions) error {
	return svc.Repo.QueryPSMs(ctx, opt)
}

func (svc *Service) UpdatePodSchedulingMetrics(ctx context.Context, operator *domain.Claims, name string, psm *domain.PodSchedulingMetrics) error {
	psmID, err := bson.ObjectIDFromHex(name)
	if err != nil {
		return errors.WithMessagef(err, "invalid PSM ID %s", name)
	}

	operatorID, err := operator.GetBsonObjectUID()
	if err != nil {
		return errors.WithMessagef(err, "invalid operator ID %s", operator.UID)
	}

	// Load existing to validate ownership.
	queryOpt := &domain.QueryPSMOptions{
		IDs: []interface{}{name},
	}
	if err := svc.Repo.QueryPSMs(ctx, queryOpt); err != nil {
		return err
	}
	if len(queryOpt.Result) == 0 {
		return errs.NewHTTPStatusError(http.StatusNotFound, "PodSchedulingMetrics not found", nil)
	}

	existing := queryOpt.Result[0]
	psm.ID = psmID
	psm.CreatedTime = existing.CreatedTime
	psm.CreatorID = existing.CreatorID
	psm.UpdaterID = operatorID

	if err := svc.Repo.UpdatePSM(ctx, psm); err != nil {
		return err
	}

	logger.Logger(ctx).Info().Msgf("updated PodSchedulingMetrics %s", name)
	return nil
}

func (svc *Service) DeletePodSchedulingMetrics(ctx context.Context, operator *domain.Claims, name string) error {
	// Validate existence.
	queryOpt := &domain.QueryPSMOptions{
		IDs: []interface{}{name},
	}
	if err := svc.Repo.QueryPSMs(ctx, queryOpt); err != nil {
		return err
	}
	if len(queryOpt.Result) == 0 {
		return errs.NewHTTPStatusError(http.StatusNotFound, "PodSchedulingMetrics not found", nil)
	}

	if err := svc.Repo.DeletePSM(ctx, name); err != nil {
		return err
	}

	logger.Logger(ctx).Info().Msgf("deleted PodSchedulingMetrics %s", name)
	return nil
}

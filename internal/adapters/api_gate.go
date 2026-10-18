package adapters

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/fireflycons/deadmanshandle/internal/ports"
)

// APIGatewayGate implements the APIGate interface on an HTTP API stage
type APIGatewayGate struct {
	client *apigatewayv2.Client
	apiID  string
	stage  string
}

// NewAPIGatewayGate creates a gate for the stage of the API
func NewAPIGatewayGate(client *apigatewayv2.Client, apiID, stage string) *APIGatewayGate {
	return &APIGatewayGate{
		client: client,
		apiID:  apiID,
		stage:  stage,
	}
}

// Throttle reads the stage's default route throttle. Terraform always sets
// one, so a stage without one is an error rather than read as closed.
func (g *APIGatewayGate) Throttle(ctx context.Context) (ports.Throttle, error) {
	settings, err := g.routeSettings(ctx)
	if err != nil {
		return ports.Throttle{}, err
	}
	if settings.ThrottlingRateLimit == nil || settings.ThrottlingBurstLimit == nil {
		return ports.Throttle{}, fmt.Errorf("stage %s of API %s has no default throttle", g.stage, g.apiID)
	}
	return ports.Throttle{
		RateLimit:  *settings.ThrottlingRateLimit,
		BurstLimit: *settings.ThrottlingBurstLimit,
	}, nil
}

// SetThrottle sets the stage's default route throttle. UpdateStage replaces
// the whole route settings, so the other settings are passed back as read.
func (g *APIGatewayGate) SetThrottle(ctx context.Context, throttle ports.Throttle) error {
	settings, err := g.routeSettings(ctx)
	if err != nil {
		return err
	}
	settings.ThrottlingRateLimit = &throttle.RateLimit
	settings.ThrottlingBurstLimit = &throttle.BurstLimit
	_, err = g.client.UpdateStage(ctx, &apigatewayv2.UpdateStageInput{
		ApiId:                &g.apiID,
		StageName:            &g.stage,
		DefaultRouteSettings: settings,
	})
	return err
}

func (g *APIGatewayGate) routeSettings(ctx context.Context) (*types.RouteSettings, error) {
	output, err := g.client.GetStage(ctx, &apigatewayv2.GetStageInput{
		ApiId:     &g.apiID,
		StageName: &g.stage,
	})
	if err != nil {
		return nil, err
	}
	if output.DefaultRouteSettings == nil {
		return &types.RouteSettings{}, nil
	}
	return output.DefaultRouteSettings, nil
}

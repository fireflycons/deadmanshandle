package adapters

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// CloudWatchAlarmStates implements the AlarmStates interface
type CloudWatchAlarmStates struct {
	client *cloudwatch.Client
}

// NewCloudWatchAlarmStates creates a new alarm state reader
func NewCloudWatchAlarmStates(client *cloudwatch.Client) *CloudWatchAlarmStates {
	return &CloudWatchAlarmStates{client: client}
}

// InAlarm reports whether the metric alarm is in the ALARM state
func (a *CloudWatchAlarmStates) InAlarm(ctx context.Context, alarmName string) (bool, error) {
	output, err := a.client.DescribeAlarms(ctx, &cloudwatch.DescribeAlarmsInput{
		AlarmNames: []string{alarmName},
		AlarmTypes: []types.AlarmType{types.AlarmTypeMetricAlarm},
	})
	if err != nil {
		return false, err
	}
	if len(output.MetricAlarms) == 0 {
		return false, fmt.Errorf("alarm %s not found", alarmName)
	}
	return output.MetricAlarms[0].StateValue == types.StateValueAlarm, nil
}

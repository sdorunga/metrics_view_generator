// AUTO-GENERATED FILE, DO NOT EDIT BY HAND
package main

import (
  "fmt"

  "go.opencensus.io/stats/view"
)

var (
  SMSSentMetricView = &view.View{
    Name: "sms/sent",
    Measure: SMSSentMetric,
    Description: "SMS Sent count",
    Aggregation: view.Count(),
  }

  PushSentMetricView = &view.View{
    Name: "push/sent",
    Measure: PushSentMetric,
    Description: "Push Sent count",
    Aggregation: view.Sum(),
  }

  EmailmetricView = &view.View{
    Name: "email/sent",
    Measure: Emailmetric,
    Description: "Email Sent count",
    Aggregation: view.Distribution(),
  }

)

func registerMetrics() error {
  if err := view.Register(SMSSentMetricView, PushSentMetricView, EmailmetricView); err != nil {
    return fmt.Errorf("failed to register transaction metrics: %w", err)
  }
  return nil
}

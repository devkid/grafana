package notifiers

import (
	"fmt"
	"net/url"

	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/log"
	"github.com/grafana/grafana/pkg/metrics"
	m "github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/alerting"
)

const PUSHOVER_ENDPOINT = "https://api.pushover.net/1/messages.json"

func init() {
	alerting.RegisterNotifier("pushover", NewPushoverNotifier)
}

func NewPushoverNotifier(model *m.AlertNotification) (alerting.Notifier, error) {
	userKey := model.Settings.Get("userKey").MustString()
	apiToken := model.Settings.Get("apiToken").MustString()

	if userKey == "" {
		return nil, alerting.ValidationError{Reason: "Could not find userkey property in settings"}
	}
	if apiToken == "" {
		return nil, alerting.ValidationError{Reason: "Could not find apikey property in settings"}
	}
	return &PushoverNotifier{
		NotifierBase: NewNotifierBase(model.Id, model.IsDefault, model.Name, model.Type, model.Settings),
		UserKey:      userKey,
		ApiToken:     apiToken,
		log:          log.New("alerting.notifier.pushover"),
	}, nil
}

type PushoverNotifier struct {
	NotifierBase
	UserKey  string
	ApiToken string
	log      log.Logger
}

func (this *PushoverNotifier) Notify(evalContext *alerting.EvalContext) error {
	metrics.M_Alerting_Notification_Sent_Pushover.Inc(1)
	ruleUrl, err := evalContext.GetRuleUrl()
	if err != nil {
		this.log.Error("Failed get rule link", "error", err)
		return err
	}
	message := evalContext.Rule.Message
	for idx, evt := range evalContext.EvalMatches {
		message += fmt.Sprintf("\n<b>%s</b>: %v", evt.Metric, evt.Value)
		if idx > 4 {
			break
		}
	}
	if evalContext.Error != nil {
		message += fmt.Sprintf("\n<b>Error message</b> %s", evalContext.Error.Error())
	}
	u, err := url.Parse(PUSHOVER_ENDPOINT)
	if err != nil {
		this.log.Error("Malformed pushover endpoint url", "error", err)
		return err
	}
	q := url.Values{}
	q.Add("user", this.UserKey)
	q.Add("token", this.ApiToken)
	q.Add("titel", evalContext.GetNotificationTitle())
	q.Add("url", ruleUrl)
	q.Add("message", message)
	q.Add("html", "1")
	u.RawQuery = q.Encode()

	cmd := &m.SendWebhookSync{Url: u.String(), HttpMethod: "POST"}

	if err := bus.DispatchCtx(evalContext.Ctx, cmd); err != nil {
		this.log.Error("Failed to send pushover notification", "error", err, "webhook", this.Name)
		return err
	}

	return nil
}

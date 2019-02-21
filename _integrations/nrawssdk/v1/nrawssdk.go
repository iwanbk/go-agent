package nrawssdk

import (
	"context"
	"reflect"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	newrelic "github.com/newrelic/go-agent"
)

type contextKeyType struct{}

var segmentContextKey = contextKeyType(struct{}{})

type endable interface{ End() error }

func getTableName(params interface{}) string {
	var tableName string

	v := reflect.ValueOf(params).Elem()
	n := v.FieldByName("TableName")
	if name, ok := n.Interface().(string); ok {
		tableName = name
	}

	return tableName
}

func startNewRelicSegment(request *request.Request) {
	httpCtx := request.HTTPRequest.Context()
	txn := newrelic.FromContext(httpCtx)

	var segment endable
	if request.ClientInfo.ServiceName == "dynamodb" {
		segment = &newrelic.DatastoreSegment{
			Product:            newrelic.DatastoreDynamoDB,
			Collection:         getTableName(request.Params),
			Operation:          request.Operation.Name,
			ParameterizedQuery: "",
			QueryParameters:    map[string]interface{}{},
			Host:               request.HTTPRequest.URL.Host,
			PortPathOrID:       request.HTTPRequest.URL.Port(),
			DatabaseName:       "",
			StartTime:          newrelic.StartSegmentNow(txn),
		}
	} else {
		segment = newrelic.StartExternalSegment(txn, request.HTTPRequest)
	}

	ctx := context.WithValue(httpCtx, segmentContextKey, segment)
	request.HTTPRequest = request.HTTPRequest.WithContext(ctx)
}

func endNewRelicSegment(request *request.Request) {
	httpCtx := request.HTTPRequest.Context()

	if segment, ok := httpCtx.Value(segmentContextKey).(endable); ok {
		segment.End()
	}
}

func InstrumentHandlers(handlers *request.Handlers) {
	handlers.Validate.SetFrontNamed(request.NamedHandler{
		Name: "startNewRelicSegment",
		Fn:   startNewRelicSegment,
	})
	handlers.Complete.SetBackNamed(request.NamedHandler{
		Name: "endNewRelicSegment",
		Fn:   endNewRelicSegment,
	})
}

func InstrumentSession(s *session.Session) *session.Session {
	InstrumentHandlers(&s.Handlers)
	return s
}

func InstrumentRequest(req *request.Request, txn newrelic.Transaction) *request.Request {
	InstrumentHandlers(&req.Handlers)
	req.HTTPRequest = newrelic.RequestWithTransactionContext(req.HTTPRequest, txn)
	return req
}

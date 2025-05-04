package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"crochet/httpclient"
	"crochet/types"
)

type ContextServiceClient struct {
	URL           string
	Client        *httpclient.GenericClient[types.ContextResponse]
	VersionClient *httpclient.GenericClient[types.VersionResponse]
	DataClient    *httpclient.GenericClient[types.ContextDataResponse]
	UpdateClient  *httpclient.GenericClient[types.VersionUpdateResponse]
}

type RemediationsServiceClient struct {
	URL          string
	Client       *httpclient.GenericClient[types.RemediationResponse]
	DeleteClient *httpclient.GenericClient[types.DeleteRemediationResponse]
	AddClient    *httpclient.GenericClient[types.AddRemediationResponse]
}

type OrthosServiceClient struct {
	URL        string
	GetClient  *httpclient.GenericClient[types.OrthosResponse]
	SaveClient *httpclient.GenericClient[types.OrthosSaveResponse]
}

type WorkServerServiceClient struct {
	URL        string
	PushClient *httpclient.GenericClient[types.WorkServerPushResponse]
	PopClient  *httpclient.GenericClient[types.WorkServerPopResponse]
	AckClient  *httpclient.GenericClient[types.WorkServerAckResponse]
}

type RabbitMQServiceClient struct {
	URL           string
	QueueName     string
	DBQueueClient *httpclient.RabbitClient[types.DBQueueItem]
}

func NewContextService(url string, client *httpclient.GenericClient[types.ContextResponse], versionClient *httpclient.GenericClient[types.VersionResponse], dataClient *httpclient.GenericClient[types.ContextDataResponse]) types.ContextService {
	updateClient := httpclient.NewDefaultGenericClient[types.VersionUpdateResponse]()

	return &ContextServiceClient{
		URL:           url,
		Client:        client,
		VersionClient: versionClient,
		DataClient:    dataClient,
		UpdateClient:  updateClient,
	}
}

func NewRemediationsService(url string, client *httpclient.GenericClient[types.RemediationResponse], deleteClient *httpclient.GenericClient[types.DeleteRemediationResponse], AddClient *httpclient.GenericClient[types.AddRemediationResponse]) types.RemediationsService {
	return &RemediationsServiceClient{
		URL:          url,
		Client:       client,
		DeleteClient: deleteClient,
		AddClient:    AddClient,
	}
}

func NewOrthosService(url string, getClient *httpclient.GenericClient[types.OrthosResponse], saveClient *httpclient.GenericClient[types.OrthosSaveResponse]) types.OrthosService {
	return &OrthosServiceClient{
		URL:        url,
		GetClient:  getClient,
		SaveClient: saveClient,
	}
}

func NewWorkServerService(url string, pushClient *httpclient.GenericClient[types.WorkServerPushResponse],
	popClient *httpclient.GenericClient[types.WorkServerPopResponse],
	ackClient *httpclient.GenericClient[types.WorkServerAckResponse]) types.WorkServerService {
	return &WorkServerServiceClient{
		URL:        url,
		PushClient: pushClient,
		PopClient:  popClient,
		AckClient:  ackClient,
	}
}

func NewRabbitMQService(url string, queueName string) (types.RabbitMQService, error) {
	dbQueueClient, err := httpclient.NewRabbitClient[types.DBQueueItem](url)
	if err != nil {
		return nil, fmt.Errorf("failed to create DB queue client: %w", err)
	}

	return &RabbitMQServiceClient{
		URL:           url,
		QueueName:     queueName,
		DBQueueClient: dbQueueClient,
	}, nil
}

func (s *ContextServiceClient) SendMessage(ctx context.Context, input types.ContextInput) (types.ContextResponse, error) {
	requestJSON, err := json.Marshal(input)
	if err != nil {
		return types.ContextResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	response, err := s.Client.GenericCall(ctx, http.MethodPost, s.URL+"/input", requestJSON)
	if err != nil {
		return types.ContextResponse{}, fmt.Errorf("service call failed: %w", err)
	}

	return response, nil
}

func (s *ContextServiceClient) GetVersion(ctx context.Context) (types.VersionResponse, error) {
	response, err := s.VersionClient.GenericCall(ctx, http.MethodGet, s.URL+"/version", nil)
	if err != nil {
		return types.VersionResponse{}, fmt.Errorf("error calling context version endpoint: %w", err)
	}

	return response, nil
}

func (s *ContextServiceClient) GetContext(ctx context.Context) (types.ContextDataResponse, error) {
	response, err := s.DataClient.GenericCall(ctx, http.MethodGet, s.URL+"/context", nil)
	if err != nil {
		return types.ContextDataResponse{}, fmt.Errorf("error calling context data endpoint: %w", err)
	}
	return response, nil
}

func (s *ContextServiceClient) UpdateVersion(ctx context.Context, request types.VersionUpdateRequest) (types.VersionUpdateResponse, error) {
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return types.VersionUpdateResponse{}, fmt.Errorf("error marshaling version update request: %w", err)
	}

	response, err := s.UpdateClient.GenericCall(ctx, http.MethodPost, s.URL+"/update-version", requestJSON)
	if err != nil {
		return types.VersionUpdateResponse{}, fmt.Errorf("error calling context update version endpoint: %w", err)
	}

	return response, nil
}

func (s *RemediationsServiceClient) FetchRemediations(ctx context.Context, request types.RemediationRequest) (types.RemediationResponse, error) {
	remediationTuples := make([]types.RemediationTuple, len(request.Pairs))
	for i, pair := range request.Pairs {
		remediationTuples[i] = types.RemediationTuple{
			Pair: pair,
		}
	}

	requestJSON, err := json.Marshal(remediationTuples)
	if err != nil {
		return types.RemediationResponse{}, fmt.Errorf("error marshaling remediation request: %w", err)
	}

	response, err := s.Client.GenericCall(ctx, http.MethodPost, s.URL+"/remediations", requestJSON)
	if err != nil {
		return types.RemediationResponse{}, fmt.Errorf("error calling remediations service: %w", err)
	}
	return response, nil
}

func (s *RemediationsServiceClient) DeleteRemediations(ctx context.Context, hashes []string) (types.DeleteRemediationResponse, error) {
	requestBody := map[string][]string{
		"hashes": hashes,
	}

	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return types.DeleteRemediationResponse{}, fmt.Errorf("error marshaling delete remediations request: %w", err)
	}

	response, err := s.DeleteClient.GenericCall(ctx, http.MethodPost, s.URL+"/delete", requestJSON)
	if err != nil {
		return types.DeleteRemediationResponse{}, fmt.Errorf("error calling remediations delete endpoint: %w", err)
	}

	return response, nil
}

func (s *RemediationsServiceClient) AddRemediations(ctx context.Context, remediations []types.RemediationTuple) (types.AddRemediationResponse, error) {
	requestJSON, err := json.Marshal(remediations)
	if err != nil {
		return types.AddRemediationResponse{}, fmt.Errorf("error marshaling add remediations request: %w", err)
	}

	response, err := s.AddClient.GenericCall(ctx, http.MethodPost, s.URL+"/remediations", requestJSON)
	if err != nil {
		return types.AddRemediationResponse{}, fmt.Errorf("error calling add remediations endpoint: %w", err)
	}

	return response, nil
}

func (s *OrthosServiceClient) GetOrthosByIDs(ctx context.Context, ids []string) (types.OrthosResponse, error) {
	requestBody := map[string][]string{
		"ids": ids,
	}

	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return types.OrthosResponse{}, fmt.Errorf("error marshaling orthos request: %w", err)
	}

	response, err := s.GetClient.GenericCall(ctx, http.MethodPost, s.URL+"/orthos/get", requestJSON)
	if err != nil {
		return types.OrthosResponse{}, fmt.Errorf("error calling orthos service: %w", err)
	}

	return response, nil
}

func (s *OrthosServiceClient) SaveOrthos(ctx context.Context, orthos []types.Ortho) (types.OrthosSaveResponse, error) {
	requestBody := map[string][]types.Ortho{
		"orthos": orthos,
	}

	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return types.OrthosSaveResponse{}, fmt.Errorf("error marshaling save orthos request: %w", err)
	}

	response, err := s.SaveClient.GenericCall(ctx, http.MethodPost, s.URL+"/orthos", requestJSON)
	if err != nil {
		return types.OrthosSaveResponse{}, fmt.Errorf("error calling orthos save endpoint: %w", err)
	}

	return response, nil
}

func (s *WorkServerServiceClient) PushOrthos(ctx context.Context, orthos []types.Ortho) (types.WorkServerPushResponse, error) {
	requestBody := map[string][]types.Ortho{
		"orthos": orthos,
	}

	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return types.WorkServerPushResponse{}, fmt.Errorf("error marshaling work server push request: %w", err)
	}

	response, err := s.PushClient.GenericCall(ctx, http.MethodPost, s.URL+"/push", requestJSON)
	if err != nil {
		return types.WorkServerPushResponse{}, fmt.Errorf("error calling work server: %w", err)
	}

	return response, nil
}

func (s *WorkServerServiceClient) Pop(ctx context.Context) (types.WorkServerPopResponse, error) {
	response, err := s.PopClient.GenericCall(ctx, http.MethodPost, s.URL+"/pop", nil)
	if err != nil {
		return types.WorkServerPopResponse{}, fmt.Errorf("error calling work server pop endpoint: %w", err)
	}

	return response, nil
}

func (s *WorkServerServiceClient) Ack(ctx context.Context, id string) (types.WorkServerAckResponse, error) {
	requestBody := map[string]string{
		"id": id,
	}

	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return types.WorkServerAckResponse{}, fmt.Errorf("error marshaling work server ack request: %w", err)
	}

	response, err := s.AckClient.GenericCall(ctx, http.MethodPost, s.URL+"/ack", requestJSON)
	if err != nil {
		return types.WorkServerAckResponse{}, fmt.Errorf("error calling work server ack endpoint: %w", err)
	}

	return response, nil
}

func (s *WorkServerServiceClient) Nack(ctx context.Context, id string) (types.WorkServerAckResponse, error) {
	requestBody := map[string]string{
		"id": id,
	}

	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return types.WorkServerAckResponse{}, fmt.Errorf("error marshaling work server nack request: %w", err)
	}

	response, err := s.AckClient.GenericCall(ctx, http.MethodPost, s.URL+"/nack", requestJSON)
	if err != nil {
		return types.WorkServerAckResponse{}, fmt.Errorf("error calling work server nack endpoint: %w", err)
	}

	return response, nil
}

func (s *RabbitMQServiceClient) PushContext(ctx context.Context, contextInput types.ContextInput) error {
	queueItem, err := types.CreateContextQueueItem(contextInput)
	if err != nil {
		return fmt.Errorf("failed to create context queue item: %w", err)
	}

	itemJSON, err := json.Marshal(queueItem)
	if err != nil {
		return fmt.Errorf("failed to marshal context queue item: %w", err)
	}

	if err := s.DBQueueClient.PushMessage(ctx, s.QueueName, itemJSON); err != nil {
		return fmt.Errorf("failed to push context to queue: %w", err)
	}

	return nil
}

func (s *RabbitMQServiceClient) PushVersion(ctx context.Context, version types.VersionInfo) error {
	queueItem, err := types.CreateVersionQueueItem(version)
	if err != nil {
		return fmt.Errorf("failed to create version queue item: %w", err)
	}

	itemJSON, err := json.Marshal(queueItem)
	if err != nil {
		return fmt.Errorf("failed to marshal version queue item: %w", err)
	}

	if err := s.DBQueueClient.PushMessage(ctx, s.QueueName, itemJSON); err != nil {
		return fmt.Errorf("failed to push version to queue: %w", err)
	}

	return nil
}

func (s *RabbitMQServiceClient) PushPairs(ctx context.Context, pairs []types.Pair) error {
	messages := make([][]byte, len(pairs))

	for i, pair := range pairs {
		queueItem, err := types.CreatePairQueueItem(pair)
		if err != nil {
			return fmt.Errorf("failed to create pair queue item at index %d: %w", i, err)
		}

		messages[i], err = json.Marshal(queueItem)
		if err != nil {
			return fmt.Errorf("failed to marshal pair queue item at index %d: %w", i, err)
		}
	}

	if err := s.DBQueueClient.PushMessageBatch(ctx, s.QueueName, messages); err != nil {
		return fmt.Errorf("failed to push pairs to queue: %w", err)
	}

	return nil
}

func (s *RabbitMQServiceClient) PushSeed(ctx context.Context, seed types.Ortho) error {
	queueItem, err := types.CreateOrthoQueueItem(seed)
	if err != nil {
		return fmt.Errorf("failed to create ortho queue item: %w", err)
	}

	itemJSON, err := json.Marshal(queueItem)
	if err != nil {
		return fmt.Errorf("failed to marshal ortho queue item: %w", err)
	}

	if err := s.DBQueueClient.PushMessage(ctx, s.QueueName, itemJSON); err != nil {
		return fmt.Errorf("failed to push seed ortho to queue: %w", err)
	}

	return nil
}

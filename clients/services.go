package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

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

type RepositoryServiceClient struct {
	URL           string
	WorkClient    *httpclient.GenericClient[types.WorkResponse]
	ContextClient *httpclient.GenericClient[types.ContextDataResponse]
	ResultsClient *httpclient.GenericClient[types.ResultsResponse]
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

// NewRabbitMQService creates a new RabbitMQ service client and declares the needed queues
func NewRabbitMQService(url string, queueName string) (types.RabbitMQService, error) {
	dbQueueClient, err := httpclient.NewRabbitClient[types.DBQueueItem](url)
	if err != nil {
		return nil, fmt.Errorf("failed to create DB queue client: %w", err)
	}

	// Declare the queue at initialization time to ensure it exists
	log.Printf("Initializing RabbitMQService and declaring queue %s", queueName)
	err = dbQueueClient.DeclareQueue(context.Background(), queueName)
	if err != nil {
		// Close the client to avoid leaks
		dbQueueClient.Close(context.Background())
		return nil, fmt.Errorf("failed to declare queue %s: %w", queueName, err)
	}
	log.Printf("Queue %s declared successfully", queueName)

	return &RabbitMQServiceClient{
		URL:           url,
		QueueName:     queueName,
		DBQueueClient: dbQueueClient,
	}, nil
}

func NewRepositoryService(url string) types.RepositoryService {
	workClient := httpclient.NewDefaultGenericClient[types.WorkResponse]()
	contextClient := httpclient.NewDefaultGenericClient[types.ContextDataResponse]()
	resultsClient := httpclient.NewDefaultGenericClient[types.ResultsResponse]()

	return &RepositoryServiceClient{
		URL:           url,
		WorkClient:    workClient,
		ContextClient: contextClient,
		ResultsClient: resultsClient,
	}
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

// PushPairs pushes pairs to the database queue in smaller batches with delays
func (s *RabbitMQServiceClient) PushPairs(ctx context.Context, pairs []types.Pair) error {
	count := len(pairs)
	if count == 0 {
		return nil
	}

	// Parse batch size from environment variable, default to 1000
	batchSizeStr := os.Getenv("FEEDER_BATCH_SIZE")
	batchSize, err := strconv.Atoi(batchSizeStr)
	if err != nil || batchSize <= 0 {
		batchSize = 1000
		log.Printf("[%s] Using default batch size: %d", ctx.Value("request_id"), batchSize)
	} else {
		log.Printf("[%s] Using configured batch size from env: %d", ctx.Value("request_id"), batchSize)
	}

	// Parse batch delay from environment variable, default to 200ms
	batchDelayStr := os.Getenv("FEEDER_PUSH_BATCH_DELAY")
	batchDelay, err := time.ParseDuration(batchDelayStr)
	if err != nil {
		batchDelay = 200 * time.Millisecond
		log.Printf("[%s] Using default batch delay: %s", ctx.Value("request_id"), batchDelay)
	} else {
		log.Printf("[%s] Using configured batch delay from env: %s", ctx.Value("request_id"), batchDelay)
	}

	log.Printf("[%s] Creating %d pair queue items in batches of %d with %s delay between batches",
		ctx.Value("request_id"), count, batchSize, batchDelay)

	// Process in batches
	for i := 0; i < count; i += batchSize {
		// Calculate end of current batch
		end := i + batchSize
		if end > count {
			end = count
		}

		currentBatch := pairs[i:end]
		log.Printf("[%s] Processing batch %d to %d of %d pairs",
			ctx.Value("request_id"), i, end-1, count)

		// Create messages for this batch
		messages := make([][]byte, len(currentBatch))
		for j, pair := range currentBatch {
			queueItem, err := types.CreatePairQueueItem(pair)
			if err != nil {
				log.Printf("[%s] Failed to create pair queue item at index %d: %v",
					ctx.Value("request_id"), i+j, err)
				return fmt.Errorf("failed to create pair queue item at index %d: %w", i+j, err)
			}

			messages[j], err = json.Marshal(queueItem)
			if err != nil {
				log.Printf("[%s] Failed to marshal pair queue item at index %d: %v",
					ctx.Value("request_id"), i+j, err)
				return fmt.Errorf("failed to marshal pair queue item at index %d: %w", i+j, err)
			}
		}

		// Push this batch
		log.Printf("[%s] Pushing batch of %d pairs to queue %s",
			ctx.Value("request_id"), len(currentBatch), s.QueueName)
		if err := s.DBQueueClient.PushMessageBatch(ctx, s.QueueName, messages); err != nil {
			log.Printf("[%s] Failed to push pair batch to queue %s: %v",
				ctx.Value("request_id"), s.QueueName, err)
			return fmt.Errorf("failed to push pair batch to queue: %w", err)
		}

		// Delay before the next batch, but only if this isn't the last batch
		if end < count {
			log.Printf("[%s] Sleeping for %s before next batch", ctx.Value("request_id"), batchDelay)
			time.Sleep(batchDelay)
		}
	}

	log.Printf("[%s] Successfully pushed %d pairs to queue %s in batches",
		ctx.Value("request_id"), count, s.QueueName)
	return nil
}

func (s *RabbitMQServiceClient) PushSeed(ctx context.Context, seed types.Ortho) error {
	log.Printf("[%s] Creating seed ortho queue item with ID %s", ctx.Value("request_id"), seed.ID)
	queueItem, err := types.CreateOrthoQueueItem(seed)
	if err != nil {
		log.Printf("[%s] Failed to create seed ortho queue item: %v", ctx.Value("request_id"), err)
		return fmt.Errorf("failed to create ortho queue item: %w", err)
	}

	log.Printf("[%s] Marshaling seed ortho queue item", ctx.Value("request_id"))
	itemJSON, err := json.Marshal(queueItem)
	if err != nil {
		log.Printf("[%s] Failed to marshal seed ortho queue item: %v", ctx.Value("request_id"), err)
		return fmt.Errorf("failed to marshal ortho queue item: %w", err)
	}

	// Queue already declared during initialization, no need to declare again
	log.Printf("[%s] Pushing seed ortho to queue %s", ctx.Value("request_id"), s.QueueName)
	if err := s.DBQueueClient.PushMessage(ctx, s.QueueName, itemJSON); err != nil {
		log.Printf("[%s] Failed to push seed ortho to queue %s: %v", ctx.Value("request_id"), s.QueueName, err)
		return fmt.Errorf("failed to push seed ortho to queue: %w", err)
	}

	log.Printf("[%s] Successfully pushed seed ortho to queue %s", ctx.Value("request_id"), s.QueueName)
	return nil
}

// PushOrtho pushes an ortho to the DB queue
func (s *RabbitMQServiceClient) PushOrtho(ctx context.Context, ortho types.Ortho) error {
	log.Printf("[%s] Creating ortho queue item with ID %s", ctx.Value("request_id"), ortho.ID)
	queueItem, err := types.CreateOrthoQueueItem(ortho)
	if err != nil {
		log.Printf("[%s] Failed to create ortho queue item: %v", ctx.Value("request_id"), err)
		return fmt.Errorf("failed to create ortho queue item: %w", err)
	}

	log.Printf("[%s] Marshaling ortho queue item", ctx.Value("request_id"))
	itemJSON, err := json.Marshal(queueItem)
	if err != nil {
		log.Printf("[%s] Failed to marshal ortho queue item: %v", ctx.Value("request_id"), err)
		return fmt.Errorf("failed to marshal ortho queue item: %w", err)
	}

	// Queue already declared during initialization, no need to declare again
	log.Printf("[%s] Pushing ortho to queue %s", ctx.Value("request_id"), s.QueueName)
	if err := s.DBQueueClient.PushMessage(ctx, s.QueueName, itemJSON); err != nil {
		log.Printf("[%s] Failed to push ortho to queue %s: %v", ctx.Value("request_id"), s.QueueName, err)
		return fmt.Errorf("failed to push ortho to queue: %w", err)
	}

	log.Printf("[%s] Successfully pushed ortho to queue %s", ctx.Value("request_id"), s.QueueName)
	return nil
}

// PushRemediationBatch pushes multiple remediations to the DB queue in smaller batches with delays
func (s *RabbitMQServiceClient) PushRemediationBatch(ctx context.Context, remediations []types.RemediationTuple) error {
	count := len(remediations)
	if count == 0 {
		return nil
	}

	// Parse batch size from environment variable, default to 1000
	batchSizeStr := os.Getenv("FEEDER_BATCH_SIZE")
	batchSize, err := strconv.Atoi(batchSizeStr)
	if err != nil || batchSize <= 0 {
		batchSize = 1000
		log.Printf("[%s] Using default batch size: %d", ctx.Value("request_id"), batchSize)
	} else {
		log.Printf("[%s] Using configured batch size from env: %d", ctx.Value("request_id"), batchSize)
	}

	// Parse batch delay from environment variable, default to 200ms
	batchDelayStr := os.Getenv("FEEDER_PUSH_BATCH_DELAY")
	batchDelay, err := time.ParseDuration(batchDelayStr)
	if err != nil {
		batchDelay = 200 * time.Millisecond
		log.Printf("[%s] Using default batch delay: %s", ctx.Value("request_id"), batchDelay)
	} else {
		log.Printf("[%s] Using configured batch delay from env: %s", ctx.Value("request_id"), batchDelay)
	}

	log.Printf("[%s] Creating %d remediation queue items in batches of %d with %s delay between batches",
		ctx.Value("request_id"), count, batchSize, batchDelay)

	// Process in batches
	for i := 0; i < count; i += batchSize {
		// Calculate end of current batch
		end := i + batchSize
		if end > count {
			end = count
		}

		currentBatch := remediations[i:end]
		log.Printf("[%s] Processing batch %d to %d of %d remediations",
			ctx.Value("request_id"), i, end-1, count)

		// Create messages for this batch
		messages := make([][]byte, len(currentBatch))
		for j, remediation := range currentBatch {
			queueItem, err := types.CreateRemediationQueueItem(remediation)
			if err != nil {
				log.Printf("[%s] Failed to create remediation queue item at index %d: %v",
					ctx.Value("request_id"), i+j, err)
				return fmt.Errorf("failed to create remediation queue item at index %d: %w", i+j, err)
			}

			messages[j], err = json.Marshal(queueItem)
			if err != nil {
				log.Printf("[%s] Failed to marshal remediation queue item at index %d: %v",
					ctx.Value("request_id"), i+j, err)
				return fmt.Errorf("failed to marshal remediation queue item at index %d: %w", i+j, err)
			}
		}

		// Push this batch
		log.Printf("[%s] Pushing batch of %d remediations to queue %s",
			ctx.Value("request_id"), len(currentBatch), s.QueueName)
		if err := s.DBQueueClient.PushMessageBatch(ctx, s.QueueName, messages); err != nil {
			log.Printf("[%s] Failed to push remediation batch to queue %s: %v",
				ctx.Value("request_id"), s.QueueName, err)
			return fmt.Errorf("failed to push remediation batch to queue: %w", err)
		}

		// Delay before the next batch, but only if this isn't the last batch
		if end < count {
			log.Printf("[%s] Sleeping for %s before next batch", ctx.Value("request_id"), batchDelay)
			time.Sleep(batchDelay)
		}
	}

	log.Printf("[%s] Successfully pushed %d remediations to queue %s in batches",
		ctx.Value("request_id"), count, s.QueueName)
	return nil
}

func (s *RabbitMQServiceClient) PushContext(ctx context.Context, contextInput types.ContextInput) error {
	log.Printf("[%s] Creating context queue item", ctx.Value("request_id"))
	queueItem, err := types.CreateContextQueueItem(contextInput)
	if err != nil {
		log.Printf("[%s] Failed to create context queue item: %v", ctx.Value("request_id"), err)
		return fmt.Errorf("failed to create context queue item: %w", err)
	}

	log.Printf("[%s] Marshaling context queue item", ctx.Value("request_id"))
	itemJSON, err := json.Marshal(queueItem)
	if err != nil {
		log.Printf("[%s] Failed to marshal context queue item: %v", ctx.Value("request_id"), err)
		return fmt.Errorf("failed to marshal context queue item: %w", err)
	}

	// Queue already declared during initialization, no need to declare again
	log.Printf("[%s] Pushing context to queue %s", ctx.Value("request_id"), s.QueueName)
	if err := s.DBQueueClient.PushMessage(ctx, s.QueueName, itemJSON); err != nil {
		log.Printf("[%s] Failed to push context to queue %s: %v", ctx.Value("request_id"), s.QueueName, err)
		return fmt.Errorf("failed to push context to queue: %w", err)
	}

	log.Printf("[%s] Successfully pushed context to queue %s", ctx.Value("request_id"), s.QueueName)
	return nil
}

// PushRemediation pushes a single remediation to the DB queue
func (s *RabbitMQServiceClient) PushRemediation(ctx context.Context, remediation types.RemediationTuple) error {
	log.Printf("[%s] Creating remediation queue item", ctx.Value("request_id"))
	queueItem, err := types.CreateRemediationQueueItem(remediation)
	if err != nil {
		log.Printf("[%s] Failed to create remediation queue item: %v", ctx.Value("request_id"), err)
		return fmt.Errorf("failed to create remediation queue item: %w", err)
	}

	log.Printf("[%s] Marshaling remediation queue item", ctx.Value("request_id"))
	itemJSON, err := json.Marshal(queueItem)
	if err != nil {
		log.Printf("[%s] Failed to marshal remediation queue item: %v", ctx.Value("request_id"), err)
		return fmt.Errorf("failed to marshal remediation queue item: %w", err)
	}

	// Queue already declared during initialization, no need to declare again
	log.Printf("[%s] Pushing remediation to queue %s", ctx.Value("request_id"), s.QueueName)
	if err := s.DBQueueClient.PushMessage(ctx, s.QueueName, itemJSON); err != nil {
		log.Printf("[%s] Failed to push remediation to queue %s: %v", ctx.Value("request_id"), s.QueueName, err)
		return fmt.Errorf("failed to push remediation to queue: %w", err)
	}

	log.Printf("[%s] Successfully pushed remediation to queue %s", ctx.Value("request_id"), s.QueueName)
	return nil
}

// PushVersion pushes a version update to the DB queue
func (s *RabbitMQServiceClient) PushVersion(ctx context.Context, version types.VersionInfo) error {
	log.Printf("[%s] Creating version queue item for version %d", ctx.Value("request_id"), version.Version)
	queueItem, err := types.CreateVersionQueueItem(version)
	if err != nil {
		log.Printf("[%s] Failed to create version queue item: %v", ctx.Value("request_id"), err)
		return fmt.Errorf("failed to create version queue item: %w", err)
	}

	log.Printf("[%s] Marshaling version queue item", ctx.Value("request_id"))
	itemJSON, err := json.Marshal(queueItem)
	if err != nil {
		log.Printf("[%s] Failed to marshal version queue item: %v", ctx.Value("request_id"), err)
		return fmt.Errorf("failed to marshal version queue item: %w", err)
	}

	// Queue already declared during initialization, no need to declare again
	log.Printf("[%s] Pushing version to queue %s", ctx.Value("request_id"), s.QueueName)
	if err := s.DBQueueClient.PushMessage(ctx, s.QueueName, itemJSON); err != nil {
		log.Printf("[%s] Failed to push version to queue %s: %v", ctx.Value("request_id"), s.QueueName, err)
		return fmt.Errorf("failed to push version to queue: %w", err)
	}

	log.Printf("[%s] Successfully pushed version %d to queue %s", ctx.Value("request_id"), version.Version, s.QueueName)
	return nil
}

func (s *RepositoryServiceClient) GetWork(ctx context.Context) (types.WorkResponse, error) {
	response, err := s.WorkClient.GenericCall(ctx, http.MethodGet, s.URL+"/work", nil)
	if err != nil {
		return types.WorkResponse{}, fmt.Errorf("error calling repository work endpoint: %w", err)
	}
	return response, nil
}

func (s *RepositoryServiceClient) GetContext(ctx context.Context) (types.ContextDataResponse, error) {
	response, err := s.ContextClient.GenericCall(ctx, http.MethodGet, s.URL+"/context", nil)
	if err != nil {
		return types.ContextDataResponse{}, fmt.Errorf("error calling repository context endpoint: %w", err)
	}
	return response, nil
}

func (s *RepositoryServiceClient) PostResults(ctx context.Context, orthos []types.Ortho, remediations []types.RemediationTuple, receipt string) (*types.ResultsResponse, error) {
	requestBody := types.ResultsRequest{
		Orthos:       orthos,
		Remediations: remediations,
		Receipt:      receipt,
	}

	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling results request: %w", err)
	}

	response, err := s.ResultsClient.GenericCall(ctx, http.MethodPost, s.URL+"/results", requestJSON)
	if err != nil {
		return nil, fmt.Errorf("error calling repository results endpoint: %w", err)
	}

	return &response, nil
}

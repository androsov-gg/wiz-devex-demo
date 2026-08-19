package cloudrun

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/logging"
	"cloud.google.com/go/logging/logadmin"
	run "cloud.google.com/go/run/apiv2"
	runpb "cloud.google.com/go/run/apiv2/runpb"
	iam "cloud.google.com/go/iam/apiv1/iampb"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/types/known/durationpb"

	"wiz-demo-platform/internal/config"
	"wiz-demo-platform/internal/models"
)

type CloudRunService struct {
	cfg       *config.Config
	runClient *run.ServicesClient
	logAdmin  *logadmin.Client
}

func New(cfg *config.Config) *CloudRunService {
	ctx := context.Background()
	rc, err := run.NewServicesClient(ctx)
	if err != nil {
		log.Printf("⚠️ Cloud Run client init warning: %v", err)
	}

	la, err := logadmin.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		log.Printf("⚠️ LogAdmin client init warning: %v", err)
	}

	return &CloudRunService{
		cfg:       cfg,
		runClient: rc,
		logAdmin:  la,
	}
}

var nonAlphanumericRegex = regexp.MustCompile(`[^a-z0-9-]`)

func sanitizeServiceID(id string) string {
	return nonAlphanumericRegex.ReplaceAllString(strings.ToLower(id), "-")
}

func (s *CloudRunService) substitutePlaceholders(input string) string {
	replacements := map[string]string{
		"PROJECT_ID": s.cfg.ProjectID,
		"REGION":     s.cfg.Region,
	}

	result := input
	for k, v := range replacements {
		result = strings.ReplaceAll(result, "${"+k+"}", v)
		result = strings.ReplaceAll(result, "${"+strings.ToLower(k)+"}", v)
		result = strings.ReplaceAll(result, "{{."+k+"}}", v)
		result = strings.ReplaceAll(result, "{{."+strings.ToLower(k)+"}}", v)
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
		result = strings.ReplaceAll(result, "{{"+strings.ToLower(k)+"}}", v)
		result = strings.ReplaceAll(result, "$"+k, v)
	}
	return result
}

func (s *CloudRunService) DeployDemoService(ctx context.Context, inst models.DemoInstance, tpl models.DemoTemplate, imageURI string) (string, error) {
	if s.runClient == nil {
		mockURL := fmt.Sprintf("https://%s-mock.a.run.app", sanitizeServiceID(inst.ID))
		return mockURL, nil
	}

	serviceID := sanitizeServiceID(inst.ID)
	parent := fmt.Sprintf("projects/%s/locations/%s", s.cfg.ProjectID, s.cfg.Region)

	port := int32(tpl.Port)
	if port == 0 {
		port = 8080
	}

	var envVars []*runpb.EnvVar
	for k, v := range tpl.DefaultEnv {
		if !strings.EqualFold(k, "PORT") {
			subKey := s.substitutePlaceholders(k)
			subVal := s.substitutePlaceholders(v)
			envVars = append(envVars, &runpb.EnvVar{
				Name:   subKey,
				Values: &runpb.EnvVar_Value{Value: subVal},
			})
		}
	}

	servicePayload := &runpb.Service{
		Ingress: runpb.IngressTraffic_INGRESS_TRAFFIC_ALL,
		Scaling: &runpb.ServiceScaling{
			ScalingMode:      runpb.ServiceScaling_MANUAL,
			MinInstanceCount: 1,
		},
		Template: &runpb.RevisionTemplate{
			ServiceAccount:  s.cfg.DemoRunnerSAEmail,
			SessionAffinity: true,
			Timeout:         durationpb.New(3600 * time.Second),
			Scaling: &runpb.RevisionScaling{
				MinInstanceCount: 1,
				MaxInstanceCount: 1,
			},
			Containers: []*runpb.Container{
				{
					Image: imageURI,
					Ports: []*runpb.ContainerPort{
						{ContainerPort: port},
					},
					Resources: &runpb.ResourceRequirements{
						Limits: map[string]string{
							"cpu":    tpl.CPU,
							"memory": tpl.Memory,
						},
					},
					Env: envVars,
				},
			},
			Labels: map[string]string{
				"app":         "wiz-demo-platform",
				"demo-id":     serviceID,
				"template-id": tpl.ID,
			},
		},
		Labels: map[string]string{
			"app":         "wiz-demo-platform",
			"demo-id":     serviceID,
			"template-id": tpl.ID,
		},
	}

	req := &runpb.CreateServiceRequest{
		Parent:    parent,
		ServiceId: serviceID,
		Service:   servicePayload,
	}

	op, err := s.runClient.CreateService(ctx, req)
	if err != nil {
		return "", fmt.Errorf("Cloud Run create service failed: %w", err)
	}

	resp, err := op.Wait(ctx)
	if err != nil {
		return "", fmt.Errorf("Cloud Run operation wait failed: %w", err)
	}

	serviceURL := resp.Uri
	if serviceURL == "" {
		serviceURL = fmt.Sprintf("https://%s-%s.%s.a.run.app", serviceID, s.cfg.ProjectID[:min(8, len(s.cfg.ProjectID))], s.cfg.Region)
	}

	_ = s.EnsureIAPConfigured(ctx, serviceID, inst.OwnerEmail)
	return serviceURL, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *CloudRunService) getHTTPClient(ctx context.Context) (*http.Client, error) {
	client, err := google.DefaultClient(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (s *CloudRunService) EnsureIAPConfigured(ctx context.Context, serviceID, ownerEmail string) error {
	client, err := s.getHTTPClient(ctx)
	if err != nil {
		log.Printf("⚠️ OAuth client creation failed for IAP patch: %v", err)
		return err
	}

	// 1. Enable IAP on Cloud Run service via REST PATCH
	patchURL := fmt.Sprintf("https://run.googleapis.com/v2/projects/%s/locations/%s/services/%s?updateMask=iapEnabled",
		s.cfg.ProjectID, s.cfg.Region, serviceID)
	bodyData, _ := json.Marshal(map[string]bool{"iapEnabled": true})
	patchReq, _ := http.NewRequestWithContext(ctx, "PATCH", patchURL, bytes.NewBuffer(bodyData))
	patchReq.Header.Set("Content-Type", "application/json")
	if patchResp, err := client.Do(patchReq); err == nil {
		patchResp.Body.Close()
	}

	// 2. Grant roles/run.invoker to the IAP service agent
	if s.runClient != nil && s.cfg.ProjectNumber != "" {
		resource := fmt.Sprintf("projects/%s/locations/%s/services/%s", s.cfg.ProjectID, s.cfg.Region, serviceID)
		policy, err := s.runClient.GetIamPolicy(ctx, &iam.GetIamPolicyRequest{Resource: resource})
		if err == nil && policy != nil {
			iapSA := fmt.Sprintf("serviceAccount:service-%s@gcp-sa-iap.iam.gserviceaccount.com", s.cfg.ProjectNumber)
			found := false
			for _, b := range policy.Bindings {
				if b.Role == "roles/run.invoker" {
					hasMember := false
					for _, m := range b.Members {
						if m == iapSA {
							hasMember = true
							break
						}
					}
					if !hasMember {
						b.Members = append(b.Members, iapSA)
					}
					found = true
					break
				}
			}
			if !found {
				policy.Bindings = append(policy.Bindings, &iam.Binding{
					Role:    "roles/run.invoker",
					Members: []string{iapSA},
				})
			}
			_, _ = s.runClient.SetIamPolicy(ctx, &iam.SetIamPolicyRequest{
				Resource: resource,
				Policy:   policy,
			})
		}
	}

	// 3. Configure IAP Web Policy for users
	members := append([]string{}, s.cfg.IAPMembers...)
	if ownerEmail != "" {
		userMember := fmt.Sprintf("user:%s", ownerEmail)
		hasOwner := false
		for _, m := range members {
			if m == userMember || m == ownerEmail {
				hasOwner = true
				break
			}
		}
		if !hasOwner {
			members = append(members, userMember)
		}
	}

	if len(members) > 0 {
		iapTarget := s.cfg.ProjectNumber
		if iapTarget == "" {
			iapTarget = s.cfg.ProjectID
		}
		iapURL := fmt.Sprintf("https://iap.googleapis.com/v1/projects/%s/iap_web/cloud_run-%s/services/%s:setIamPolicy",
			iapTarget, s.cfg.Region, serviceID)

		iapPayload := map[string]interface{}{
			"policy": map[string]interface{}{
				"bindings": []map[string]interface{}{
					{
						"role":    "roles/iap.httpsResourceAccessor",
						"members": members,
					},
				},
			},
		}

		payloadBytes, _ := json.Marshal(iapPayload)
		iapReq, _ := http.NewRequestWithContext(ctx, "POST", iapURL, bytes.NewBuffer(payloadBytes))
		iapReq.Header.Set("Content-Type", "application/json")
		if iapResp, err := client.Do(iapReq); err == nil {
			iapResp.Body.Close()
		}
	}

	return nil
}

func (s *CloudRunService) GetDemoServiceStatus(ctx context.Context, instanceID string) (models.InstanceStatus, string, bool, error) {
	if s.runClient == nil {
		return models.StatusRunning, fmt.Sprintf("https://%s-mock.a.run.app", sanitizeServiceID(instanceID)), true, nil
	}

	serviceID := sanitizeServiceID(instanceID)
	name := fmt.Sprintf("projects/%s/locations/%s/services/%s", s.cfg.ProjectID, s.cfg.Region, serviceID)

	svc, err := s.runClient.GetService(ctx, &runpb.GetServiceRequest{Name: name})
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "NOT_FOUND") {
			return models.StatusStopped, "", false, nil
		}
		return models.StatusProvisioning, "", false, err
	}

	serviceURL := svc.Uri
	terminalCond := svc.TerminalCondition
	var readyCond *runpb.Condition
	for _, c := range svc.Conditions {
		if c.Type == "Ready" {
			readyCond = c
			break
		}
	}
	if readyCond == nil {
		readyCond = terminalCond
	}

	isSucceeded := false
	if terminalCond != nil && terminalCond.State == runpb.Condition_CONDITION_SUCCEEDED {
		isSucceeded = true
	}
	if readyCond != nil && readyCond.State == runpb.Condition_CONDITION_SUCCEEDED {
		isSucceeded = true
	}
	if !svc.Reconciling && serviceURL != "" && (terminalCond == nil || terminalCond.State != runpb.Condition_CONDITION_FAILED) {
		isSucceeded = true
	}

	isFailed := false
	if terminalCond != nil && terminalCond.State == runpb.Condition_CONDITION_FAILED {
		isFailed = true
	}
	if readyCond != nil && readyCond.State == runpb.Condition_CONDITION_FAILED {
		isFailed = true
	}

	if isSucceeded {
		return models.StatusRunning, serviceURL, true, nil
	}
	if isFailed {
		return models.StatusFailed, serviceURL, false, nil
	}

	return models.StatusProvisioning, serviceURL, false, nil
}

func (s *CloudRunService) DeleteDemoService(ctx context.Context, instanceID string) error {
	if s.runClient == nil {
		return nil
	}

	serviceID := sanitizeServiceID(instanceID)
	name := fmt.Sprintf("projects/%s/locations/%s/services/%s", s.cfg.ProjectID, s.cfg.Region, serviceID)

	op, err := s.runClient.DeleteService(ctx, &runpb.DeleteServiceRequest{Name: name})
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "NOT_FOUND") {
			return nil
		}
		return err
	}

	_, _ = op.Wait(ctx)
	return nil
}

func (s *CloudRunService) RestartDemoService(ctx context.Context, instanceID, ownerEmail string) error {
	if s.runClient == nil {
		return nil
	}

	serviceID := sanitizeServiceID(instanceID)
	name := fmt.Sprintf("projects/%s/locations/%s/services/%s", s.cfg.ProjectID, s.cfg.Region, serviceID)

	svc, err := s.runClient.GetService(ctx, &runpb.GetServiceRequest{Name: name})
	if err != nil {
		return err
	}

	if svc.Template == nil {
		svc.Template = &runpb.RevisionTemplate{}
	}
	if svc.Template.Annotations == nil {
		svc.Template.Annotations = make(map[string]string)
	}

	svc.Template.Annotations["wiz.dev/restartedAt"] = time.Now().Format(time.RFC3339)

	op, err := s.runClient.UpdateService(ctx, &runpb.UpdateServiceRequest{
		Service: svc,
	})
	if err != nil {
		return err
	}

	_, err = op.Wait(ctx)
	if err != nil {
		return err
	}

	_ = s.EnsureIAPConfigured(ctx, serviceID, ownerEmail)
	return nil
}

func (s *CloudRunService) GetDemoServiceLogs(ctx context.Context, instanceID string, lines int) (string, error) {
	if s.logAdmin == nil {
		return "Simulated container logs: Service running normally.", nil
	}

	serviceID := sanitizeServiceID(instanceID)
	filter := fmt.Sprintf("resource.type=\"cloud_run_revision\" AND resource.labels.service_name=\"%s\"", serviceID)

	iter := s.logAdmin.Entries(ctx,
		logadmin.Filter(filter),
		logadmin.NewestFirst(),
	)

	var entries []*logging.Entry
	for {
		entry, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			break
		}
		entries = append(entries, entry)
		if len(entries) >= lines {
			break
		}
	}

	if len(entries) == 0 {
		return "No logs found yet for this Cloud Run service.", nil
	}

	// Reverse to show oldest first
	var logLines []string
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		msg := ""
		switch payload := e.Payload.(type) {
		case string:
			msg = payload
		case map[string]interface{}:
			if m, ok := payload["message"].(string); ok {
				msg = m
			} else {
				jsonBytes, _ := json.Marshal(payload)
				msg = string(jsonBytes)
			}
		default:
			msg = fmt.Sprintf("%v", payload)
		}

		timePrefix := ""
		if !e.Timestamp.IsZero() {
			timePrefix = fmt.Sprintf("[%s] ", e.Timestamp.Format(time.RFC3339))
		}
		logLines = append(logLines, timePrefix+msg)
	}

	return strings.Join(logLines, "\n"), nil
}

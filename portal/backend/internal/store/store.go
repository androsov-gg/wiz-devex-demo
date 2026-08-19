package store

import (
	"context"
	"log"
	"sort"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"

	"wiz-demo-platform/internal/config"
	"wiz-demo-platform/internal/models"
)

type Store struct {
	cfg       *config.Config
	firestore *firestore.Client

	mu        sync.RWMutex
	memInst   map[string]models.DemoInstance
	memBuilds map[string]models.BuildRecord
}

func New(cfg *config.Config) *Store {
	ctx := context.Background()
	var fc *firestore.Client
	var err error

	if cfg.FirestoreDBID != "" && cfg.FirestoreDBID != "(default)" {
		fc, err = firestore.NewClientWithDatabase(ctx, cfg.ProjectID, cfg.FirestoreDBID)
	} else {
		fc, err = firestore.NewClient(ctx, cfg.ProjectID)
	}

	if err != nil {
		log.Printf("⚠️ Firestore initialization failed for database %q, using in-memory store: %v", cfg.FirestoreDBID, err)
	}

	return &Store{
		cfg:       cfg,
		firestore: fc,
		memInst:   make(map[string]models.DemoInstance),
		memBuilds: make(map[string]models.BuildRecord),
	}
}

func (s *Store) SaveInstance(ctx context.Context, inst models.DemoInstance) error {
	if s.firestore != nil {
		_, err := s.firestore.Collection("instances").Doc(inst.ID).Set(ctx, inst, firestore.MergeAll)
		if err == nil {
			return nil
		}
		log.Printf("Firestore write failed, fallback to memory: %v", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.memInst[inst.ID] = inst
	return nil
}

func (s *Store) GetInstance(ctx context.Context, id string) (*models.DemoInstance, error) {
	if s.firestore != nil {
		doc, err := s.firestore.Collection("instances").Doc(id).Get(ctx)
		if err == nil && doc.Exists() {
			var inst models.DemoInstance
			if err := doc.DataTo(&inst); err == nil {
				return &inst, nil
			}
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if inst, ok := s.memInst[id]; ok {
		return &inst, nil
	}
	return nil, nil
}

func (s *Store) ListInstances(ctx context.Context) ([]models.DemoInstance, error) {
	if s.firestore != nil {
		iter := s.firestore.Collection("instances").OrderBy("createdAt", firestore.Desc).Documents(ctx)
		var list []models.DemoInstance
		for {
			doc, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				log.Printf("Firestore list error: %v", err)
				break
			}
			var inst models.DemoInstance
			if err := doc.DataTo(&inst); err == nil {
				list = append(list, inst)
			}
		}
		if len(list) > 0 {
			return list, nil
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []models.DemoInstance
	for _, inst := range s.memInst {
		list = append(list, inst)
	}
	sort.Slice(list, func(i, j int) bool {
		tA, _ := time.Parse(time.RFC3339, list[i].CreatedAt)
		tB, _ := time.Parse(time.RFC3339, list[j].CreatedAt)
		return tA.After(tB)
	})
	return list, nil
}

func (s *Store) UpdateInstanceStatus(ctx context.Context, id string, status models.InstanceStatus, updates map[string]interface{}) error {
	payload := map[string]interface{}{
		"status": status,
	}
	for k, v := range updates {
		payload[k] = v
	}

	if s.firestore != nil {
		var firestoreUpdates []firestore.Update
		for k, v := range payload {
			firestoreUpdates = append(firestoreUpdates, firestore.Update{Path: k, Value: v})
		}
		_, err := s.firestore.Collection("instances").Doc(id).Update(ctx, firestoreUpdates)
		if err == nil {
			return nil
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if inst, ok := s.memInst[id]; ok {
		inst.Status = status
		if val, exists := updates["serviceUrl"]; exists {
			inst.ServiceURL = val.(string)
		}
		if val, exists := updates["cloudRunReadyAt"]; exists {
			inst.CloudRunReadyAt = val.(string)
		}
		if val, exists := updates["errorMessage"]; exists {
			inst.ErrorMessage = val.(string)
		}
		s.memInst[id] = inst
	}
	return nil
}

func (s *Store) DeleteInstance(ctx context.Context, id string) error {
	if s.firestore != nil {
		_, err := s.firestore.Collection("instances").Doc(id).Delete(ctx)
		if err != nil {
			log.Printf("Firestore delete failed: %v", err)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.memInst, id)
	return nil
}

func (s *Store) SaveBuildRecord(ctx context.Context, build models.BuildRecord) error {
	if s.firestore != nil {
		_, err := s.firestore.Collection("builds").Doc(build.BuildID).Set(ctx, build, firestore.MergeAll)
		if err == nil {
			return nil
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.memBuilds[build.BuildID] = build
	return nil
}

func (s *Store) GetBuildRecord(ctx context.Context, buildID string) (*models.BuildRecord, error) {
	if s.firestore != nil {
		doc, err := s.firestore.Collection("builds").Doc(buildID).Get(ctx)
		if err == nil && doc.Exists() {
			var rec models.BuildRecord
			if err := doc.DataTo(&rec); err == nil {
				return &rec, nil
			}
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if rec, ok := s.memBuilds[buildID]; ok {
		return &rec, nil
	}
	return nil, nil
}

package clusterconf

import (
	"encoding/json"
	"net/url"
	"path"
	"strings"

	"github.com/cerana/cerana/acomm"
	"github.com/cerana/cerana/pkg/errors"
	"github.com/pborman/uuid"
)

const servicesPrefix string = "services"

// Service is information about a service.
type Service struct {
	ServiceConf
	c *ClusterConf
	// ModIndex should be treated as opaque, but passed back on updates
	ModIndex uint64 `json:"modIndex"`
}

// ServiceConf is the configuration of a service.
type ServiceConf struct {
	ID           string                 `json:"id"`
	Dataset      string                 `json:"dataset"`
	HealthChecks map[string]HealthCheck `json:"healthChecks"`
	Limits       ResourceLimits         `json:"limits"`
	Env          map[string]string      `json:"env"`
	Cmd          []string               `json:"cmd"`
}

// ResourceLimits is configuration for resource upper bounds.
type ResourceLimits struct {
	CPU       int   `json:"cpu"`
	Memory    int64 `json:"memory"`
	Processes int   `json:"processes"`
}

// HealthCheck is configuration for performing a health check.
type HealthCheck struct {
	ID   string      `json:"id"`
	Type string      `json:"type"`
	Args interface{} `json:"args"`
}

// ServicePayload can be used for task args or result when a service object
// needs to be sent.
type ServicePayload struct {
	Service *Service `json:"service"`
}

// GetService retrieves a service.
func (c *ClusterConf) GetService(req *acomm.Request) (interface{}, *url.URL, error) {
	var args IDArgs
	if err := req.UnmarshalArgs(&args); err != nil {
		return nil, nil, err
	}
	if args.ID == "" {
		return nil, nil, errors.Newv("missing arg: id", map[string]interface{}{"args": args})
	}

	service, err := c.getService(args.ID)
	if err != nil {
		return nil, nil, err
	}
	return &ServicePayload{service}, nil, nil
}

// UpdateService creates or updates a service config. When updating, a Get should first be performed and the modified Service passed back.
func (c *ClusterConf) UpdateService(req *acomm.Request) (interface{}, *url.URL, error) {
	var args ServicePayload
	if err := req.UnmarshalArgs(&args); err != nil {
		return nil, nil, err
	}
	if args.Service == nil {
		return nil, nil, errors.Newv("missing arg: service", map[string]interface{}{"args": args})
	}
	args.Service.c = c

	if args.Service.ID == "" {
		args.Service.ID = uuid.New()
	}

	if err := args.Service.update(); err != nil {
		return nil, nil, err
	}
	return &ServicePayload{args.Service}, nil, nil
}

// DeleteService deletes a service config.
func (c *ClusterConf) DeleteService(req *acomm.Request) (interface{}, *url.URL, error) {
	var args IDArgs
	if err := req.UnmarshalArgs(&args); err != nil {
		return nil, nil, err
	}
	if args.ID == "" {
		return nil, nil, errors.Newv("missing arg: id", map[string]interface{}{"args": args})
	}

	service, err := c.getService(args.ID)
	if err != nil {
		if strings.Contains(err.Error(), "service config not found") {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	return nil, nil, service.delete()
}

func (c *ClusterConf) getService(id string) (*Service, error) {
	service := &Service{
		c:           c,
		ServiceConf: ServiceConf{ID: id},
	}
	if err := service.reload(); err != nil {
		return nil, err
	}
	return service, nil
}

func (s *Service) reload() error {
	key := path.Join(servicesPrefix, s.ID, "config")
	value, err := s.c.kvGet(key)
	if err != nil {
		if strings.Contains(err.Error(), "key not found") {
			err = errors.Newv("service config not found", map[string]interface{}{"serviceID": s.ID})
		}
		return err
	}

	if err := json.Unmarshal(value.Data, &s.ServiceConf); err != nil {
		return errors.Wrapv(err, map[string]interface{}{"json": string(value.Data)})
	}
	s.ModIndex = value.Index
	return nil
}

func (s *Service) delete() error {
	key := path.Join(servicesPrefix, s.ID)
	return errors.Wrapv(s.c.kvDelete(key, s.ModIndex), map[string]interface{}{"serviceID": s.ID})
}

// update saves the service config.
func (s *Service) update() error {
	key := path.Join(servicesPrefix, s.ID, "config")

	modIndex, err := s.c.kvUpdate(key, s.ServiceConf, s.ModIndex)
	if err != nil {
		return errors.Wrapv(err, map[string]interface{}{"serviceID": s.ID})
	}
	s.ModIndex = modIndex

	return nil
}

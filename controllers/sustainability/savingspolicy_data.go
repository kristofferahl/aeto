package sustainability

import (
	"fmt"
	"strconv"
	"time"

	sustainabilityv1alpha1 "github.com/kristofferahl/aeto/apis/sustainability/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

const (
	secretKeyLastTransitionTime = "lastTransitionTime"
	secretKeySuspended          = "suspended"
	secretKeyReason             = "reason"
	secretKeySuspendedDuration  = "suspendedDuration"
	secretKeyDeployments        = "deployments"
)

type SavingsPolicyData struct {
	LastTransitionTime time.Time
	Suspended          bool
	Reason             string
	SuspendedDuration  time.Duration
	DeploymentsInfo    []DeploymentReplicas
}

func NewSavingsPolicyData(secret *v1.Secret, savingsPolicy sustainabilityv1alpha1.SavingsPolicy, suspended bool, reason string) (SavingsPolicyData, error) {
	timestamp := time.Now().UTC()
	savingsPolicyData := &SavingsPolicyData{
		LastTransitionTime: timestamp,
		Suspended:          suspended,
		Reason:             reason,
		SuspendedDuration:  0 * time.Second,
	}

	if secret == nil || secret.Data == nil {
		return *savingsPolicyData, nil
	}

	err := savingsPolicyData.LoadFromSecretData(secret.Data)
	if err != nil {
		return SavingsPolicyData{}, fmt.Errorf("failed to set resource info in SavingsPolicyData %s: %s", savingsPolicy.Name, err)
	}

	if suspended != savingsPolicyData.Suspended {
		if !suspended {
			sd := savingsPolicyData.SuspendedDuration
			since := savingsPolicyData.TimeSinceLastTransition()
			if since > 0*time.Second {
				sd = sd + since
			}
			savingsPolicyData.SuspendedDuration = sd
		}

		savingsPolicyData.LastTransitionTime = timestamp
	}

	savingsPolicyData.Suspended = suspended
	savingsPolicyData.Reason = reason

	return *savingsPolicyData, nil
}

func (s *SavingsPolicyData) LoadFromSecretData(data map[string][]byte) error {
	lastTranstionTime, err := time.Parse(time.RFC3339, string(data[secretKeyLastTransitionTime]))
	if err != nil {
		return err
	}
	s.LastTransitionTime = lastTranstionTime

	suspended, err := strconv.ParseBool(string(data[secretKeySuspended]))
	if err != nil {
		return err
	}
	s.Suspended = suspended

	reason := string(data[secretKeyReason])
	s.Reason = reason

	suspendedDuration, err := time.ParseDuration(string(data[secretKeySuspendedDuration]))
	if err != nil {
		return err
	}
	s.SuspendedDuration = suspendedDuration

	deploymentsInfo, err := ConvertToDeploymentsInfo(data[secretKeyDeployments])
	if err != nil {
		return err
	}
	s.DeploymentsInfo = deploymentsInfo

	return nil
}

func (s *SavingsPolicyData) NewSecretData(resources Resources) (stringData map[string]string, data map[string][]byte, err error) {
	stringData = make(map[string]string)
	data = make(map[string][]byte)

	stringData[secretKeyLastTransitionTime] = s.LastTransitionTime.Format(time.RFC3339)
	stringData[secretKeySuspended] = strconv.FormatBool(s.Suspended)
	stringData[secretKeyReason] = s.Reason
	stringData[secretKeySuspendedDuration] = s.SuspendedDuration.String()

	if resources.HasResources() && !s.Suspended {
		ri, err := resources.Info()
		if err != nil {
			return stringData, data, err
		}
		data = ri
	}

	return
}

func (s *SavingsPolicyData) TimeSinceLastTransition() time.Duration {
	if s.LastTransitionTime.IsZero() {
		return 0 * time.Second
	}

	return time.Now().UTC().Sub(s.LastTransitionTime)
}

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
)

// Clients wrapper for AWS
type Clients struct {
	Route53 *route53.Client
}

// GetRoute53HostedZoneById returns Route53 HostedZone by ID
func (c Clients) GetRoute53HostedZoneById(ctx context.Context, id string) (route53types.HostedZone, error) {
	res, err := c.Route53.GetHostedZone(ctx, &route53.GetHostedZoneInput{
		Id: aws.String(id),
	})
	if err != nil {
		return route53types.HostedZone{}, err
	}

	return *res.HostedZone, nil
}

// GetRoute53HostedZoneByName returns the first matching Route53 HostedZone by name
func (c Clients) GetRoute53HostedZoneByName(ctx context.Context, name string) (route53types.HostedZone, error) {
	zones, err := c.Route53.ListHostedZonesByName(ctx, &route53.ListHostedZonesByNameInput{
		DNSName: aws.String(name),
	})
	if err != nil {
		return route53types.HostedZone{}, err
	}

	for _, zone := range zones.HostedZones {
		isMatch := *zone.Name == name+"."
		if isMatch {
			return zone, nil
		}
	}

	return route53types.HostedZone{}, fmt.Errorf("no AWS Route53 HostedZone found matching the name %s", name)
}

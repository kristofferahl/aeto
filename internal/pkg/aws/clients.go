package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/aws-sdk-go/aws"
)

// Clients wrapper for AWS
type Clients struct {
	Route53 *route53.Client
}

// GetRoute53HostedZoneByID gets a Route53 HostedZone by ID
func (c Clients) GetRoute53HostedZoneByID(ctx context.Context, id string) (types.HostedZone, error) {
	res, err := c.Route53.GetHostedZone(ctx, &route53.GetHostedZoneInput{
		Id: aws.String(id),
	})
	if err != nil {
		return types.HostedZone{}, err
	}

	return *res.HostedZone, nil
}

// GetRoute53HostedZoneByName gets the first matching Route53 HostedZone by name
func (c Clients) GetRoute53HostedZoneByName(ctx context.Context, name string) (types.HostedZone, error) {
	zones, err := c.Route53.ListHostedZonesByName(ctx, &route53.ListHostedZonesByNameInput{
		DNSName: aws.String(name),
	})
	if err != nil {
		return types.HostedZone{}, err
	}

	for _, zone := range zones.HostedZones {
		isMatch := *zone.Name == name+"."
		if isMatch {
			return zone, nil
		}
	}

	return types.HostedZone{}, fmt.Errorf("no AWS Route53 HostedZone found matching the name %s", name)
}

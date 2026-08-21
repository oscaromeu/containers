package allquiet

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

// Field names follow the public OpenAPI spec (/api/swagger/public-v1/swagger.json).

// Incident is one incident as the search endpoints return it. There is no
// top-level status: the current status is whatever the newest event set.
type Incident struct {
	ID                           string      `json:"id"`
	Title                        string      `json:"title"`
	CreatedAt                    *time.Time  `json:"createdAt"`
	LastUpdatedAt                time.Time   `json:"lastUpdatedAt"`
	Attributes                   []Attribute `json:"attributes"`
	Events                       []Event     `json:"events"`
	EventsTotalCount             int         `json:"eventsTotalCount"`
	Services                     []Entity    `json:"services"`
	Teams                        []Entity    `json:"teams"`
	IsArchived                   bool        `json:"isArchived"`
	ExcludeFromUptimeCalculation bool        `json:"excludeFromUptimeCalculation"`
}

// Entity is a referenced team or service.
type Entity struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

// Attribute is one key/value the inbound integration mapped from the alert.
type Attribute struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Event is one entry of the incident timeline.
type Event struct {
	Severity     *string      `json:"severity"`
	Status       *string      `json:"status"`
	Message      string       `json:"message"`
	Modification Modification `json:"modification"`
}

// Modification says who did what, and when.
type Modification struct {
	Timestamp time.Time `json:"timestamp"`
	Intent    string    `json:"intent"`
	User      *User     `json:"user"`
}

// User is the human behind a modification.
type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}

// AttributeValue returns the value of the named attribute, or "".
func (i Incident) AttributeValue(name string) string {
	for _, a := range i.Attributes {
		if a.Name == name {
			return a.Value
		}
	}
	return ""
}

// SearchParams filter the incident listing.
type SearchParams struct {
	TeamIDs         []string
	LastUpdatedFrom time.Time
	Limit           int
}

type incidentList struct {
	Incidents []Incident `json:"incidents"`
	HasMore   bool       `json:"hasMore"`
}

// SearchIncidents pages through /incident/search/list until hasMore is false.
// The API includes archived incidents by default (the UI hides them); that is
// load-bearing here: Archive without affects is how a false positive is recorded.
func (c *Client) SearchIncidents(ctx context.Context, p SearchParams) ([]Incident, error) {
	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}
	var all []Incident
	offset := 0
	for {
		q := url.Values{}
		for _, id := range p.TeamIDs {
			q.Add("TeamIds", id)
		}
		if !p.LastUpdatedFrom.IsZero() {
			q.Set("LastUpdatedFrom", p.LastUpdatedFrom.UTC().Format(time.RFC3339))
		}
		q.Set("Limit", strconv.Itoa(limit))
		q.Set("Offset", strconv.Itoa(offset))

		var page incidentList
		if err := c.get(ctx, "/incident/search/list", q, &page); err != nil {
			return nil, err
		}
		all = append(all, page.Incidents...)
		if !page.HasMore {
			return all, nil
		}
		offset += limit
	}
}

// GetIncident fetches one incident with its full timeline; the listing may
// truncate events (eventsTotalCount > len(events)).
func (c *Client) GetIncident(ctx context.Context, id string) (*Incident, error) {
	var inc Incident
	if err := c.get(ctx, "/incident/search/"+url.PathEscape(id), nil, &inc); err != nil {
		return nil, err
	}
	return &inc, nil
}

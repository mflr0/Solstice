package api

// The current user's filter subscriptions, all filters together: the page
// "My settings > Subscriptions" lists them, where the per-filter routes
// (notifications.go) only answer for one filter at a time.

import (
	"net/http"
	"sort"
	"strings"
)

type mySubscriptionOut struct {
	subscriptionOut
	FilterName string `json:"filterName"`
}

// mySubscriptions lists the subscriptions the user receives (their own, or
// of a group they belong to) or created, on the filters they can still see,
// by filter name. Administrators see only their own here, not everyone's.
func (s *Server) mySubscriptions(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	filters, err := s.loadFilters(ctx, sub, 0)
	if err != nil {
		return err
	}
	out := []mySubscriptionOut{}
	for _, f := range filters {
		list, err := s.out.notify.Subscriptions(ctx, f.out.ID)
		if err != nil {
			return err
		}
		rows, err := s.subscriptionsOut(ctx, sub, list)
		if err != nil {
			return err
		}
		for _, o := range rows {
			mine := o.User != nil && o.User.ID == sub.UserID || o.Owner != nil && o.Owner.ID == sub.UserID
			if mine || o.Group != nil && sub.InGroup(o.Group.ID) {
				out = append(out, mySubscriptionOut{subscriptionOut: o, FilterName: f.out.Name})
			}
		}
	}
	sort.SliceStable(out, func(a, b int) bool {
		if na, nb := strings.ToLower(out[a].FilterName), strings.ToLower(out[b].FilterName); na != nb {
			return na < nb
		}
		return out[a].ID < out[b].ID
	})
	return writeJSON(w, http.StatusOK, out)
}

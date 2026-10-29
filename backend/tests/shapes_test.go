package tests

import "time"

// The API's answers, as far as the tests read them. They are written here
// rather than imported: the tests check the JSON contract, not Go types.

type userRef struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Active   bool   `json:"active"`
}

type idName struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type namedRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type typeRef struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subtask bool   `json:"subtask"`
}

type statusRef struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

type projectOut struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
}

type catalog struct {
	DefaultResolution string `json:"defaultResolution"`
}

type counter struct {
	Count int  `json:"count"`
	Me    bool `json:"me"`
}

type issueDetail struct {
	ID         int64      `json:"id"`
	Key        string     `json:"key"`
	Summary    string     `json:"summary"`
	Type       typeRef    `json:"type"`
	Status     statusRef  `json:"status"`
	Priority   *namedRef  `json:"priority"`
	Resolution *namedRef  `json:"resolution"`
	Assignee   *userRef   `json:"assignee"`
	Reporter   *userRef   `json:"reporter"`
	Resolved   *time.Time `json:"resolved"`
	Project    struct {
		Key string `json:"key"`
	} `json:"project"`
	Time            timeTracking `json:"time"`
	Sprints         []idName     `json:"sprints"`
	Labels          []string     `json:"labels"`
	Components      []idName     `json:"components"`
	AffectsVersions []idName     `json:"affectsVersions"`
	FixVersions     []idName     `json:"fixVersions"`
	Links           []struct {
		ID    int64 `json:"id"`
		Issue struct {
			Key string `json:"key"`
		} `json:"issue"`
	} `json:"links"`
	Attachments []struct {
		ID  int64  `json:"id"`
		URL string `json:"url"`
	} `json:"attachments"`
	CustomFields []customValue   `json:"customFields"`
	Watchers     counter         `json:"watchers"`
	Votes        counter         `json:"votes"`
	Can          map[string]bool `json:"can"`
	Statuses     []statusRef     `json:"statuses"`
	Transitions  []struct {
		ID int64     `json:"id"`
		To statusRef `json:"to"`
	} `json:"transitions"`
	Epic *struct {
		Key string `json:"key"`
	} `json:"epic"`
}

type customValue struct {
	ID     int64    `json:"id"`
	Name   string   `json:"name"`
	Kind   string   `json:"kind"`
	Values []string `json:"values"`
}

type commentOut struct {
	ID         int64    `json:"id"`
	Author     *userRef `json:"author"`
	Body       string   `json:"body"`
	Visibility *struct {
		Group string `json:"group"`
		Role  string `json:"role"`
	} `json:"visibility"`
	CanEdit bool `json:"canEdit"`
}

type historyEntry struct {
	Items []struct {
		Field string  `json:"field"`
		From  *string `json:"from"`
		To    *string `json:"to"`
	} `json:"items"`
}

type filterOut struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Mine      bool   `json:"mine"`
	Favourite bool   `json:"favourite"`
	CanEdit   bool   `json:"canEdit"`
}

type tokenOut struct {
	ID       int64      `json:"id"`
	Name     string     `json:"name"`
	LastUsed *time.Time `json:"lastUsed"`
	Expired  bool       `json:"expired"`
	Scopes   []string   `json:"scopes"`
	User     *userRef   `json:"user"`
	Token    string     `json:"token"`
}

type prefsOut struct {
	Delivery    string          `json:"delivery"`
	OwnChanges  bool            `json:"ownChanges"`
	Events      map[string]bool `json:"events"`
	DueReminder bool            `json:"dueReminder"`
	MailEnabled bool            `json:"mailEnabled"`
}

type subscriptionOut struct {
	ID      int64     `json:"id"`
	User    *userRef  `json:"user"`
	Group   *idName   `json:"group"`
	Hour    int       `json:"hour"`
	Days    []int     `json:"days"`
	NextRun time.Time `json:"nextRun"`
	CanEdit bool      `json:"canEdit"`
}

type mailSettingsOut struct {
	Host        string  `json:"host"`
	Username    string  `json:"username"`
	FromName    string  `json:"fromName"`
	PasswordSet bool    `json:"passwordSet"`
	BaseURL     *string `json:"baseUrl"`
}

type auditEntryOut struct {
	User     *userRef       `json:"user"`
	Username *string        `json:"username"`
	IP       *string        `json:"ip"`
	Action   string         `json:"action"`
	Target   *auditTarget   `json:"target"`
	Details  map[string]any `json:"details"`
}

type auditTarget struct {
	Type  string  `json:"type"`
	ID    *string `json:"id"`
	Label *string `json:"label"`
}

type grantOut struct {
	Permission string `json:"permission"`
	Holder     string `json:"holder"`
	ID         int64  `json:"id,omitempty"`
	Name       string `json:"name,omitempty"`
}

type updateStateOut struct {
	Current       string     `json:"current"`
	Latest        *string    `json:"latest"`
	Newer         bool       `json:"newer"`
	CheckedAt     *time.Time `json:"checkedAt"`
	CheckError    *string    `json:"checkError"`
	ChecksEnabled bool       `json:"checksEnabled"`
	Install       string     `json:"install"`
	CanApply      bool       `json:"canApply"`
	Commands      []string   `json:"commands"`
	Download      string     `json:"download"`
	LastAttempt   *struct {
		From   string `json:"from"`
		To     string `json:"to"`
		Status string `json:"status"`
	} `json:"lastAttempt"`
}

type timeTracking struct {
	Original  *int64 `json:"original"`
	Remaining *int64 `json:"remaining"`
	Spent     *int64 `json:"spent"`
}

type sprintOut struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	State     string     `json:"state"`
	Goal      *string    `json:"goal"`
	Start     *time.Time `json:"start"`
	End       *time.Time `json:"end"`
	Completed *time.Time `json:"completed"`
}

type worklogOut struct {
	ID      int64     `json:"id"`
	Author  *userRef  `json:"author"`
	Started time.Time `json:"started"`
	Seconds int64     `json:"seconds"`
	Body    *string   `json:"body"`
	CanEdit bool      `json:"canEdit"`
}

type statusUsageOut struct {
	Workflows []idName `json:"workflows"`
	Boards    []struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		Column string `json:"column"`
	} `json:"boards"`
	Projects []struct {
		Key    string `json:"key"`
		Name   string `json:"name"`
		Issues int    `json:"issues"`
	} `json:"projects"`
}

type mySubscriptionOut struct {
	ID         int64    `json:"id"`
	FilterID   int64    `json:"filterId"`
	FilterName string   `json:"filterName"`
	User       *userRef `json:"user"`
	Schedule   string   `json:"schedule"`
	CanEdit    bool     `json:"canEdit"`
}

package i18n

// Requests: addresses, sign-in, headers, internal errors.
const (
	RequestUnknownAddress       Key = "RequestUnknownAddress"
	RequestNotFound             Key = "RequestNotFound"
	RequestForbidden            Key = "RequestForbidden"
	RequestSignIn               Key = "RequestSignIn"
	RequestCredentialsRefused   Key = "RequestCredentialsRefused"
	RequestMissingHeader        Key = "RequestMissingHeader"
	RequestXSRFHeader           Key = "RequestXSRFHeader"
	RequestInvalid              Key = "RequestInvalid"
	RequestTooManyAttempts      Key = "RequestTooManyAttempts"
	RequestStreamingUnsupported Key = "RequestStreamingUnsupported"
	RequestInternalError        Key = "RequestInternalError"
)

// Values shared by several forms.
const (
	CommonNameRequired    Key = "CommonNameRequired"
	CommonAlreadyExists   Key = "CommonAlreadyExists"
	CommonDateInvalid     Key = "CommonDateInvalid"
	CommonOrderInvalid    Key = "CommonOrderInvalid"
	CommonTimeZoneUnknown Key = "CommonTimeZoneUnknown"
)

// Accounts, passwords and the profile.
const (
	AccountSignInFailed              Key = "AccountSignInFailed"
	AccountNotFound                  Key = "AccountNotFound"
	AccountUsernameInvalid           Key = "AccountUsernameInvalid"
	AccountUsernameTaken             Key = "AccountUsernameTaken"
	AccountEmailInvalid              Key = "AccountEmailInvalid"
	AccountOwnAdminRights            Key = "AccountOwnAdminRights"
	AccountOwnPasswordInProfile      Key = "AccountOwnPasswordInProfile"
	AccountPasswordTooShort          Key = "AccountPasswordTooShort"
	AccountPasswordIsUsername        Key = "AccountPasswordIsUsername"
	AccountCurrentPasswordRequired   Key = "AccountCurrentPasswordRequired"
	AccountCurrentPasswordIncorrect  Key = "AccountCurrentPasswordIncorrect"
	AccountTokenCannotChangePassword Key = "AccountTokenCannotChangePassword"
	AccountLanguageUnsupported       Key = "AccountLanguageUnsupported"
)

// Access tokens.
const (
	TokenCannotCreate       Key = "TokenCannotCreate"
	TokenNameRequired       Key = "TokenNameRequired"
	TokenExpiryInvalid      Key = "TokenExpiryInvalid" // #nosec G101 -- message key, not a credential
	TokenExpiryPast         Key = "TokenExpiryPast"    // #nosec G101 -- message key, not a credential
	TokenCreationRestricted Key = "TokenCreationRestricted"
	TokenPolicyInvalid      Key = "TokenPolicyInvalid"
)

// Groups.
const (
	GroupNotFound   Key = "GroupNotFound"
	GroupExists     Key = "GroupExists"
	GroupRestricted Key = "GroupRestricted"
)

// Project roles.
const (
	RoleNotFound   Key = "RoleNotFound"
	RoleExists     Key = "RoleExists"
	RoleRestricted Key = "RoleRestricted"
)

// Instance settings.
const (
	SettingsNameRequired          Key = "SettingsNameRequired"
	SettingsBannerLevelUnknown    Key = "SettingsBannerLevelUnknown"
	SettingsBannerTooLong         Key = "SettingsBannerTooLong"
	SettingsSessionLengthInvalid  Key = "SettingsSessionLengthInvalid"
	SettingsPasswordLengthInvalid Key = "SettingsPasswordLengthInvalid"
)

// Audit log.
const (
	AuditRetentionInvalid  Key = "AuditRetentionInvalid"
	AuditRetentionTooShort Key = "AuditRetentionTooShort"
)

// Setup: setup code, database, public address, first administrator.
const (
	SetupNotDone              Key = "SetupNotDone"
	SetupDone                 Key = "SetupDone"
	SetupCodeRequired         Key = "SetupCodeRequired"
	SetupCodeIncorrect        Key = "SetupCodeIncorrect"
	SetupHostInvalid          Key = "SetupHostInvalid"
	SetupPortInvalid          Key = "SetupPortInvalid"
	SetupDatabaseNameInvalid  Key = "SetupDatabaseNameInvalid"
	SetupDatabaseUserRequired Key = "SetupDatabaseUserRequired"
	SetupDatabaseTooOld       Key = "SetupDatabaseTooOld"
	SetupDatabaseCharset      Key = "SetupDatabaseCharset"
	SetupDatabaseNotEmpty     Key = "SetupDatabaseNotEmpty"
	SetupDatabaseAuth         Key = "SetupDatabaseAuth"
	SetupDatabaseAccess       Key = "SetupDatabaseAccess"
	SetupDatabaseMissing      Key = "SetupDatabaseMissing"
	SetupDatabaseConnect      Key = "SetupDatabaseConnect"
	SetupDatabaseUnreachable  Key = "SetupDatabaseUnreachable"
	SetupDatabaseFromEnv      Key = "SetupDatabaseFromEnv"
	SetupDatabaseImported     Key = "SetupDatabaseImported"
	SetupDatabaseRequired     Key = "SetupDatabaseRequired"
	SetupTablesFailed         Key = "SetupTablesFailed"
	SetupPublicURLInvalid     Key = "SetupPublicURLInvalid"
	SetupAccountNotImported   Key = "SetupAccountNotImported"
)

// Import from Jira.
const (
	ImportMaintenance         Key = "ImportMaintenance"
	ImportRunning             Key = "ImportRunning"
	ImportNotStarted          Key = "ImportNotStarted"
	ImportNotFinished         Key = "ImportNotFinished"
	ImportFailed              Key = "ImportFailed"
	ImportConfirmDatabase     Key = "ImportConfirmDatabase"
	ImportDatabaseURLInvalid  Key = "ImportDatabaseURLInvalid"
	ImportOptionRefused       Key = "ImportOptionRefused"
	ImportOptionInvalid       Key = "ImportOptionInvalid"
	ImportDatabaseAuth        Key = "ImportDatabaseAuth"
	ImportDatabaseMissing     Key = "ImportDatabaseMissing"
	ImportDatabaseOpen        Key = "ImportDatabaseOpen"
	ImportDatabaseUnreachable Key = "ImportDatabaseUnreachable"
	ImportNotJira             Key = "ImportNotJira"
	ImportHomeNotAbsolute     Key = "ImportHomeNotAbsolute"
	ImportHomeNotFound        Key = "ImportHomeNotFound"
	ImportNoAttachments       Key = "ImportNoAttachments"
)

// Updates of Solstice.
const (
	UpdatePasswordIncorrect Key = "UpdatePasswordIncorrect"
	UpdateNotFromInterface  Key = "UpdateNotFromInterface"
	UpdateNotLatest         Key = "UpdateNotLatest"
	UpdateUpToDate          Key = "UpdateUpToDate"
	UpdateInProgress        Key = "UpdateInProgress"
)

// Projects and their permissions.
const (
	ProjectNotFound            Key = "ProjectNotFound"
	ProjectUnknown             Key = "ProjectUnknown"
	ProjectKeyInvalid          Key = "ProjectKeyInvalid"
	ProjectKeyTaken            Key = "ProjectKeyTaken"
	ProjectNameTaken           Key = "ProjectNameTaken"
	ProjectLeadInactive        Key = "ProjectLeadInactive"
	ProjectCategoryNotFound    Key = "ProjectCategoryNotFound"
	ProjectTemplateUnknown     Key = "ProjectTemplateUnknown"
	ProjectAssigneeTypeUnknown Key = "ProjectAssigneeTypeUnknown"
	ProjectIssueTypeRequired   Key = "ProjectIssueTypeRequired"
	ProjectRemovedTypesInUse   Key = "ProjectRemovedTypesInUse"
	ProjectDeleteConfirm       Key = "ProjectDeleteConfirm"
	ProjectGranteeUnknown      Key = "ProjectGranteeUnknown"
	ProjectGranteeNotFound     Key = "ProjectGranteeNotFound"
)

// Data a new instance starts with: issue types, priorities, resolutions, link types, roles.
const (
	DefaultTypeTask                  Key = "DefaultTypeTask"
	DefaultTypeBug                   Key = "DefaultTypeBug"
	DefaultTypeStory                 Key = "DefaultTypeStory"
	DefaultTypeEpic                  Key = "DefaultTypeEpic"
	DefaultTypeSubtask               Key = "DefaultTypeSubtask"
	DefaultPriorityVeryHigh          Key = "DefaultPriorityVeryHigh"
	DefaultPriorityHigh              Key = "DefaultPriorityHigh"
	DefaultPriorityMedium            Key = "DefaultPriorityMedium"
	DefaultPriorityLow               Key = "DefaultPriorityLow"
	DefaultPriorityVeryLow           Key = "DefaultPriorityVeryLow"
	DefaultResolutionDone            Key = "DefaultResolutionDone"
	DefaultResolutionWontDo          Key = "DefaultResolutionWontDo"
	DefaultResolutionDuplicate       Key = "DefaultResolutionDuplicate"
	DefaultResolutionCannotReproduce Key = "DefaultResolutionCannotReproduce"
	DefaultLinkBlocksName            Key = "DefaultLinkBlocksName"
	DefaultLinkBlocksInward          Key = "DefaultLinkBlocksInward"
	DefaultLinkBlocksOutward         Key = "DefaultLinkBlocksOutward"
	DefaultLinkRelatesName           Key = "DefaultLinkRelatesName"
	DefaultLinkRelatesDirection      Key = "DefaultLinkRelatesDirection"
	DefaultLinkDuplicatesName        Key = "DefaultLinkDuplicatesName"
	DefaultLinkDuplicatesInward      Key = "DefaultLinkDuplicatesInward"
	DefaultLinkDuplicatesOutward     Key = "DefaultLinkDuplicatesOutward"
	DefaultLinkClonesName            Key = "DefaultLinkClonesName"
	DefaultLinkClonesInward          Key = "DefaultLinkClonesInward"
	DefaultLinkClonesOutward         Key = "DefaultLinkClonesOutward"
	DefaultRoleAdministrators        Key = "DefaultRoleAdministrators"
	DefaultRoleDevelopers            Key = "DefaultRoleDevelopers"
	DefaultRoleUsers                 Key = "DefaultRoleUsers"
)

// Permission names, as the REST API lists them.
const (
	PermissionBrowse      Key = "PermissionBrowse"
	PermissionCreate      Key = "PermissionCreate"
	PermissionEdit        Key = "PermissionEdit"
	PermissionComment     Key = "PermissionComment"
	PermissionTransition  Key = "PermissionTransition"
	PermissionAssign      Key = "PermissionAssign"
	PermissionAssignable  Key = "PermissionAssignable"
	PermissionAttach      Key = "PermissionAttach"
	PermissionLink        Key = "PermissionLink"
	PermissionWorklog     Key = "PermissionWorklog"
	PermissionDelete      Key = "PermissionDelete"
	PermissionModerate    Key = "PermissionModerate"
	PermissionWatchers    Key = "PermissionWatchers"
	PermissionSecurity    Key = "PermissionSecurity"
	PermissionAdmin       Key = "PermissionAdmin"
	PermissionGlobalAdmin Key = "PermissionGlobalAdmin"
	PermissionUnavailable Key = "PermissionUnavailable"
)

// Catalogues: issue types, priorities, resolutions, link types.
const (
	CatalogColorInvalid        Key = "CatalogColorInvalid"
	CatalogTypeSubtaskLocked   Key = "CatalogTypeSubtaskLocked"
	CatalogLinkTypeIncomplete  Key = "CatalogLinkTypeIncomplete"
	CatalogLastIssueType       Key = "CatalogLastIssueType"
	CatalogReplacementNotFound Key = "CatalogReplacementNotFound"
	CatalogReplacementKind     Key = "CatalogReplacementKind"
	CatalogItemInUse           Key = "CatalogItemInUse"
)

// Custom fields administration.
const (
	CustomFieldTypeUnknown   Key = "CustomFieldTypeUnknown"
	CustomFieldNoOptions     Key = "CustomFieldNoOptions"
	CustomFieldOptionTooLong Key = "CustomFieldOptionTooLong"
	CustomFieldUsedByBoards  Key = "CustomFieldUsedByBoards"
	CustomFieldDeleteConfirm Key = "CustomFieldDeleteConfirm"
	CustomFieldOptionInUse   Key = "CustomFieldOptionInUse"
	CustomFieldOptionMoveTo  Key = "CustomFieldOptionMoveTo"
)

// Field values of an issue.
const (
	FieldUnknown              Key = "FieldUnknown"
	FieldRequired             Key = "FieldRequired"
	FieldValueInvalid         Key = "FieldValueInvalid"
	FieldValueUnknown         Key = "FieldValueUnknown"
	FieldValueNotInProject    Key = "FieldValueNotInProject"
	FieldTooLong              Key = "FieldTooLong"
	FieldNotOffered           Key = "FieldNotOffered"
	FieldNotSettable          Key = "FieldNotSettable"
	FieldOperationUnsupported Key = "FieldOperationUnsupported"
)

// Field names, as the REST API, exports and e-mails show them.
const (
	FieldNameKey               Key = "FieldNameKey"
	FieldNameSummary           Key = "FieldNameSummary"
	FieldNameIssueType         Key = "FieldNameIssueType"
	FieldNameProject           Key = "FieldNameProject"
	FieldNameStatus            Key = "FieldNameStatus"
	FieldNamePriority          Key = "FieldNamePriority"
	FieldNameResolution        Key = "FieldNameResolution"
	FieldNameResolved          Key = "FieldNameResolved"
	FieldNameAssignee          Key = "FieldNameAssignee"
	FieldNameReporter          Key = "FieldNameReporter"
	FieldNameCreator           Key = "FieldNameCreator"
	FieldNameCreated           Key = "FieldNameCreated"
	FieldNameUpdated           Key = "FieldNameUpdated"
	FieldNameDueDate           Key = "FieldNameDueDate"
	FieldNameDescription       Key = "FieldNameDescription"
	FieldNameEnvironment       Key = "FieldNameEnvironment"
	FieldNameLabels            Key = "FieldNameLabels"
	FieldNameComponent         Key = "FieldNameComponent"
	FieldNameComponents        Key = "FieldNameComponents"
	FieldNameFixVersion        Key = "FieldNameFixVersion"
	FieldNameFixVersions       Key = "FieldNameFixVersions"
	FieldNameAffectsVersion    Key = "FieldNameAffectsVersion"
	FieldNameAffectsVersions   Key = "FieldNameAffectsVersions"
	FieldNameLink              Key = "FieldNameLink"
	FieldNameLinkedIssues      Key = "FieldNameLinkedIssues"
	FieldNameSubtasks          Key = "FieldNameSubtasks"
	FieldNameParent            Key = "FieldNameParent"
	FieldNameEpic              Key = "FieldNameEpic"
	FieldNameSprint            Key = "FieldNameSprint"
	FieldNameSecurityLevel     Key = "FieldNameSecurityLevel"
	FieldNameWatchers          Key = "FieldNameWatchers"
	FieldNameVotes             Key = "FieldNameVotes"
	FieldNameOriginalEstimate  Key = "FieldNameOriginalEstimate"
	FieldNameRemainingEstimate Key = "FieldNameRemainingEstimate"
	FieldNameTimeSpent         Key = "FieldNameTimeSpent"
	FieldNameTimeTracking      Key = "FieldNameTimeTracking"
	FieldNameAttachment        Key = "FieldNameAttachment"
	FieldNameAttachments       Key = "FieldNameAttachments"
	FieldNameComments          Key = "FieldNameComments"
	FieldNameWorkLog           Key = "FieldNameWorkLog"
)

// Components.
const (
	ComponentAssigneeTypeUnknown Key = "ComponentAssigneeTypeUnknown"
	ComponentReplacementNotFound Key = "ComponentReplacementNotFound"
)

// Versions.
const (
	VersionStartAfterRelease Key = "VersionStartAfterRelease"
	VersionTargetNotFound    Key = "VersionTargetNotFound"
	VersionMoveToUnreleased  Key = "VersionMoveToUnreleased"
)

// Issues: creation, editing, hierarchy, assignment, deletion.
const (
	IssueTypeUnknown           Key = "IssueTypeUnknown"
	IssueTypeNotInProject      Key = "IssueTypeNotInProject"
	IssueTypeUnavailable       Key = "IssueTypeUnavailable"
	IssueSummaryRequired       Key = "IssueSummaryRequired"
	IssueSummaryEmpty          Key = "IssueSummaryEmpty"
	IssuePriorityUnknown       Key = "IssuePriorityUnknown"
	IssueResolutionUnknown     Key = "IssueResolutionUnknown"
	IssueDueDateInvalid        Key = "IssueDueDateInvalid"
	IssueEstimateInvalid       Key = "IssueEstimateInvalid"
	IssueSecurityLevelUnknown  Key = "IssueSecurityLevelUnknown"
	IssueReporterFixed         Key = "IssueReporterFixed"
	IssueNoTodoStatus          Key = "IssueNoTodoStatus"
	IssueTransitionNotAllowed  Key = "IssueTransitionNotAllowed"
	IssueTransitionUnavailable Key = "IssueTransitionUnavailable"
	IssueParentNotFound        Key = "IssueParentNotFound"
	IssueParentInvalid         Key = "IssueParentInvalid"
	IssueParentNotSubtask      Key = "IssueParentNotSubtask"
	IssueSubtaskParentRequired Key = "IssueSubtaskParentRequired"
	IssueSubtaskTypeChange     Key = "IssueSubtaskTypeChange"
	IssueSubtaskSecurity       Key = "IssueSubtaskSecurity"
	IssueSubtaskEpic           Key = "IssueSubtaskEpic"
	IssueEpicNotFound          Key = "IssueEpicNotFound"
	IssueEpicInEpic            Key = "IssueEpicInEpic"
	IssueNotAnEpic             Key = "IssueNotAnEpic"
	IssueAssignOthersForbidden Key = "IssueAssignOthersForbidden"
	IssueAssigneeNotAssignable Key = "IssueAssigneeNotAssignable"
	IssueNotVisibleTo          Key = "IssueNotVisibleTo"
	IssueMoveUnsupported       Key = "IssueMoveUnsupported"
	IssueSubtasksNotDeletable  Key = "IssueSubtasksNotDeletable"
	IssueDeleteSubtasks        Key = "IssueDeleteSubtasks"
)

// Comments and their visibility.
const (
	CommentEmpty           Key = "CommentEmpty"
	CommentGroupNotMember  Key = "CommentGroupNotMember"
	CommentRoleNotHeld     Key = "CommentRoleNotHeld"
	CommentVisibilityFixed Key = "CommentVisibilityFixed"
)

// Work logs.
const (
	WorkLogDurationInvalid Key = "WorkLogDurationInvalid"
	WorkLogRestricted      Key = "WorkLogRestricted"
)

// Issue links.
const (
	LinkTypeUnknown    Key = "LinkTypeUnknown"
	LinkTargetNotFound Key = "LinkTargetNotFound"
	LinkToItself       Key = "LinkToItself"
)

// Attachments.
const (
	AttachmentUploadInvalid Key = "AttachmentUploadInvalid"
	AttachmentNoFile        Key = "AttachmentNoFile"
	AttachmentTooLarge      Key = "AttachmentTooLarge"
	AttachmentNotImported   Key = "AttachmentNotImported"
	AttachmentFileMissing   Key = "AttachmentFileMissing"
)

// Statuses and their categories.
const (
	StatusUnknown            Key = "StatusUnknown"
	StatusNameUnknown        Key = "StatusNameUnknown"
	StatusNameRequired       Key = "StatusNameRequired"
	StatusNameTaken          Key = "StatusNameTaken"
	StatusCategoryInvalid    Key = "StatusCategoryInvalid"
	StatusInUse              Key = "StatusInUse"
	StatusCategoryToDo       Key = "StatusCategoryToDo"
	StatusCategoryInProgress Key = "StatusCategoryInProgress"
	StatusCategoryDone       Key = "StatusCategoryDone"
)

// Workflows.
const (
	WorkflowNameRequired           Key = "WorkflowNameRequired"
	WorkflowStatusRequired         Key = "WorkflowStatusRequired"
	WorkflowStatusTwice            Key = "WorkflowStatusTwice"
	WorkflowInitialStatus          Key = "WorkflowInitialStatus"
	WorkflowTransitionNameRequired Key = "WorkflowTransitionNameRequired"
	WorkflowTransitionStatus       Key = "WorkflowTransitionStatus"
	WorkflowTransitionNoTarget     Key = "WorkflowTransitionNoTarget"
	WorkflowTransitionUnknown      Key = "WorkflowTransitionUnknown"
	WorkflowStatusesRemoved        Key = "WorkflowStatusesRemoved"
	WorkflowCopyNotFound           Key = "WorkflowCopyNotFound"
	WorkflowDefaultDelete          Key = "WorkflowDefaultDelete"
	WorkflowDefaultStatusesMissing Key = "WorkflowDefaultStatusesMissing"
)

// Boards, their settings and ranking.
const (
	BoardTypeUnknown             Key = "BoardTypeUnknown"
	BoardNameRequired            Key = "BoardNameRequired"
	BoardQueryInvalid            Key = "BoardQueryInvalid"
	BoardSubFilter               Key = "BoardSubFilter"
	BoardPeriodInvalid           Key = "BoardPeriodInvalid"
	BoardSwimlanesUnknown        Key = "BoardSwimlanesUnknown"
	BoardEstimationUnknown       Key = "BoardEstimationUnknown"
	BoardColumnRequired          Key = "BoardColumnRequired"
	BoardColumnNameRequired      Key = "BoardColumnNameRequired"
	BoardColumnLimits            Key = "BoardColumnLimits"
	BoardColumnUnknown           Key = "BoardColumnUnknown"
	BoardStatusInTwoColumns      Key = "BoardStatusInTwoColumns"
	BoardQuickFilterNameRequired Key = "BoardQuickFilterNameRequired"
	BoardQuickFilterNamed        Key = "BoardQuickFilterNamed"
	BoardSwimlaneNameRequired    Key = "BoardSwimlaneNameRequired"
	BoardSwimlaneNamed           Key = "BoardSwimlaneNamed"
	BoardMoveNotAllowed          Key = "BoardMoveNotAllowed"
	BoardIssueCount              Key = "BoardIssueCount"
	BoardRankTargetRequired      Key = "BoardRankTargetRequired"
	BoardRankItself              Key = "BoardRankItself"
)

// Sprints.
const (
	SprintKanban          Key = "SprintKanban"
	SprintUnknown         Key = "SprintUnknown"
	SprintStarted         Key = "SprintStarted"
	SprintAlreadyActive   Key = "SprintAlreadyActive"
	SprintEndPast         Key = "SprintEndPast"
	SprintNotActive       Key = "SprintNotActive"
	SprintTargetInvalid   Key = "SprintTargetInvalid"
	SprintNameRequired    Key = "SprintNameRequired"
	SprintClosedLocked    Key = "SprintClosedLocked"
	SprintDatesOrder      Key = "SprintDatesOrder"
	SprintStartFuture     Key = "SprintStartFuture"
	SprintDeleteNotFuture Key = "SprintDeleteNotFuture"
)

// Saved filters and their shares.
const (
	FilterNameRequired        Key = "FilterNameRequired"
	FilterIncomplete          Key = "FilterIncomplete"
	FilterShareKindUnknown    Key = "FilterShareKindUnknown"
	FilterShareGroupNotMember Key = "FilterShareGroupNotMember"
	FilterShareRoleNotHeld    Key = "FilterShareRoleNotHeld"
	FilterSharePersonNotFound Key = "FilterSharePersonNotFound"
)

// Exports.
const (
	ExportColumnUnknown Key = "ExportColumnUnknown"
)

// JQL: syntax and query errors.
const (
	JQLError                 Key = "JQLError"
	JQLInvalid               Key = "JQLInvalid"
	JQLUnclosedQuote         Key = "JQLUnclosedQuote"
	JQLIncompleteBang        Key = "JQLIncompleteBang"
	JQLUnexpectedChar        Key = "JQLUnexpectedChar"
	JQLUnexpected            Key = "JQLUnexpected"
	JQLOrderBy               Key = "JQLOrderBy"
	JQLSortFieldExpected     Key = "JQLSortFieldExpected"
	JQLSortUnsupported       Key = "JQLSortUnsupported"
	JQLParenExpected         Key = "JQLParenExpected"
	JQLTooLong               Key = "JQLTooLong"
	JQLTooDeep               Key = "JQLTooDeep"
	JQLFieldExpected         Key = "JQLFieldExpected"
	JQLNotIn                 Key = "JQLNotIn"
	JQLOperatorExpected      Key = "JQLOperatorExpected"
	JQLIsEmpty               Key = "JQLIsEmpty"
	JQLChangedOnly           Key = "JQLChangedOnly"
	JQLDuringDates           Key = "JQLDuringDates"
	JQLArgumentInvalid       Key = "JQLArgumentInvalid"
	JQLValueExpected         Key = "JQLValueExpected"
	JQLFieldUnknown          Key = "JQLFieldUnknown"
	JQLFieldUnsupported      Key = "JQLFieldUnsupported"
	JQLOperatorField         Key = "JQLOperatorField"
	JQLOperatorDate          Key = "JQLOperatorDate"
	JQLOperatorDuration      Key = "JQLOperatorDuration"
	JQLFunctionMisplaced     Key = "JQLFunctionMisplaced"
	JQLFunctionUser          Key = "JQLFunctionUser"
	JQLFunctionSprint        Key = "JQLFunctionSprint"
	JQLFunctionVersion       Key = "JQLFunctionVersion"
	JQLMembersOfGroup        Key = "JQLMembersOfGroup"
	JQLStatusCategoryUnknown Key = "JQLStatusCategoryUnknown"
	JQLDateFunctionUnknown   Key = "JQLDateFunctionUnknown"
	JQLDateInvalid           Key = "JQLDateInvalid"
	JQLOffsetInvalid         Key = "JQLOffsetInvalid"
	JQLDurationInvalid       Key = "JQLDurationInvalid"
	JQLIssueKeyInvalid       Key = "JQLIssueKeyInvalid"
	JQLNumberExpected        Key = "JQLNumberExpected"
	JQLVersionExpected       Key = "JQLVersionExpected"
	JQLEmptyHistory          Key = "JQLEmptyHistory"
	JQLDateExpected          Key = "JQLDateExpected"
	JQLByUser                Key = "JQLByUser"
	JQLHistoryField          Key = "JQLHistoryField"
)

// E-mail server settings and the test e-mail.
const (
	MailDisabled              Key = "MailDisabled"
	MailServerRequired        Key = "MailServerRequired"
	MailPortInvalid           Key = "MailPortInvalid"
	MailSecurityUnknown       Key = "MailSecurityUnknown"
	MailSenderInvalid         Key = "MailSenderInvalid"
	MailRecipientInvalid      Key = "MailRecipientInvalid"
	MailSettingsIncomplete    Key = "MailSettingsIncomplete"
	MailPasswordUnreadable    Key = "MailPasswordUnreadable"
	MailPasswordEncryptFailed Key = "MailPasswordEncryptFailed"
	MailPasswordRequired      Key = "MailPasswordRequired"
	MailSendFailed            Key = "MailSendFailed"
	MailConnectFailed         Key = "MailConnectFailed"
	MailTLSFailed             Key = "MailTLSFailed"
	MailNoStartTLS            Key = "MailNoStartTLS"
	MailAuthRefused           Key = "MailAuthRefused"
	MailAuthUnsupported       Key = "MailAuthUnsupported"
	MailSenderRefused         Key = "MailSenderRefused"
	MailRecipientRefused      Key = "MailRecipientRefused"
	MailMessageRefused        Key = "MailMessageRefused"
	MailTestSubject           Key = "MailTestSubject"
	MailTestBody              Key = "MailTestBody"
	MailTestSettings          Key = "MailTestSettings"
	MailTestWhy               Key = "MailTestWhy"
)

// Notification preferences and notification e-mails.
const (
	NotifyHourInvalid         Key = "NotifyHourInvalid"
	NotifyDeliveryUnknown     Key = "NotifyDeliveryUnknown"
	NotifyEventUnknown        Key = "NotifyEventUnknown"
	NotifyRecipientUnknown    Key = "NotifyRecipientUnknown"
	NotifyManage              Key = "NotifyManage"
	NotifyChangeInPreferences Key = "NotifyChangeInPreferences"
	NotifyOpenIssue           Key = "NotifyOpenIssue"
	NotifyEmptyValue          Key = "NotifyEmptyValue"
	NotifyRestrictedComment   Key = "NotifyRestrictedComment"
	NotifySomeone             Key = "NotifySomeone"
	NotifyAssignedTo          Key = "NotifyAssignedTo"
	NotifyUnassigned          Key = "NotifyUnassigned"
	NotifyDigestSubject       Key = "NotifyDigestSubject"
	NotifyWhyDigest           Key = "NotifyWhyDigest"
	NotifyWhyAssignee         Key = "NotifyWhyAssignee"
	NotifyWhyReporter         Key = "NotifyWhyReporter"
	NotifyWhyMention          Key = "NotifyWhyMention"
	NotifyWhyWatcher          Key = "NotifyWhyWatcher"
	NotifyWhyProject          Key = "NotifyWhyProject"
	NotifyCreated             Key = "NotifyCreated"
	NotifyDeleted             Key = "NotifyDeleted"
	NotifyUpdated             Key = "NotifyUpdated"
	NotifyResolved            Key = "NotifyResolved"
	NotifyReopened            Key = "NotifyReopened"
	NotifyAssignedToYou       Key = "NotifyAssignedToYou"
	NotifyCommented           Key = "NotifyCommented"
	NotifyCommentEdited       Key = "NotifyCommentEdited"
	NotifyMentionComment      Key = "NotifyMentionComment"
	NotifyMentionDescription  Key = "NotifyMentionDescription"
)

// Filter subscriptions and their e-mails.
const (
	SubscriptionDayRequired      Key = "SubscriptionDayRequired"
	SubscriptionDaysInvalid      Key = "SubscriptionDaysInvalid"
	SubscriptionFrequencyUnknown Key = "SubscriptionFrequencyUnknown"
	SubscriptionGroupAdminOnly   Key = "SubscriptionGroupAdminOnly"
	SubscriptionSubject          Key = "SubscriptionSubject"
	SubscriptionCount            Key = "SubscriptionCount"
	SubscriptionNoMatch          Key = "SubscriptionNoMatch"
	SubscriptionTruncated        Key = "SubscriptionTruncated"
	SubscriptionOpen             Key = "SubscriptionOpen"
	SubscriptionWhyGroup         Key = "SubscriptionWhyGroup"
	SubscriptionWhyUser          Key = "SubscriptionWhyUser"
)

// Due date reminders.
const (
	ReminderSubject  Key = "ReminderSubject"
	ReminderOverdue  Key = "ReminderOverdue"
	ReminderDueToday Key = "ReminderDueToday"
	ReminderWhy      Key = "ReminderWhy"
)

// Webhooks.
const (
	WebhookNameRequired        Key = "WebhookNameRequired"
	WebhookURLTooLong          Key = "WebhookURLTooLong"
	WebhookURLForbidden        Key = "WebhookURLForbidden"
	WebhookURLScheme           Key = "WebhookURLScheme"
	WebhookURLNoHost           Key = "WebhookURLNoHost"
	WebhookEventRequired       Key = "WebhookEventRequired"
	WebhookSecretTooLong       Key = "WebhookSecretTooLong"
	WebhookSecretEncryptFailed Key = "WebhookSecretEncryptFailed"
)

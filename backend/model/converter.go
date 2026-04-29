package model

import (
	"time"

	"github.com/serverhub/serverhub/domain"
	"gorm.io/gorm"
)

// ── Server ──────────────────────────────────────────────────────────────

func ToDomainServer(s Server) domain.Server {
	networks := make([]domain.Network, len(s.Networks))
	for i, n := range s.Networks {
		networks[i] = domain.Network{
			Kind:          n.Kind,
			NetworkID:     n.NetworkID,
			Address:       n.Address,
			Priority:      n.Priority,
			ReachableFrom: n.ReachableFrom,
			Label:         n.Label,
		}
	}
	return domain.Server{
		ID:         s.ID,
		Name:       s.Name,
		Type:       s.Type,
		Host:       s.Host,
		Port:       s.Port,
		Username:   s.Username,
		AuthType:   s.AuthType,
		Password:   s.Password,
		PrivateKey: s.PrivateKey,
		Remark:     s.Remark,
		HostKeyFP:  s.HostKeyFP,
		Capability: s.Capability,
		Networks:   networks,
		CreatedAt:  s.CreatedAt,
		UpdatedAt:  s.UpdatedAt,
	}
}

func FromDomainServer(d domain.Server) Server {
	networks := make(Networks, len(d.Networks))
	for i, n := range d.Networks {
		networks[i] = Network{
			Kind:          n.Kind,
			NetworkID:     n.NetworkID,
			Address:       n.Address,
			Priority:      n.Priority,
			ReachableFrom: n.ReachableFrom,
			Label:         n.Label,
		}
	}
	return Server{
		ID:         d.ID,
		Name:       d.Name,
		Type:       d.Type,
		Host:       d.Host,
		Port:       d.Port,
		Username:   d.Username,
		AuthType:   d.AuthType,
		Password:   d.Password,
		PrivateKey: d.PrivateKey,
		Remark:     d.Remark,
		HostKeyFP:  d.HostKeyFP,
		Capability: d.Capability,
		Networks:   networks,
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
	}
}

// ── Service ────────────────────────────────────────────────────────────

func ToDomainService(s Service) domain.Service {
	return domain.Service{
		ID:                 s.ID,
		Name:               s.Name,
		ServerID:           s.ServerID,
		Type:               domain.ServiceType(s.Type),
		ApplicationID:      s.ApplicationID,
		WorkDir:            s.WorkDir,
		ExposedPort:        s.ExposedPort,
		WebhookSecret:      s.WebhookSecret,
		CurrentReleaseID:   s.CurrentReleaseID,
		AutoRollbackOnFail: s.AutoRollbackOnFail,
		AutoSync:           s.AutoSync,
		SyncInterval:       s.SyncInterval,
		SyncStatus:         s.SyncStatus,
		SourceKind:         s.SourceKind,
		SourceID:           s.SourceID,
		SourceFingerprint:  s.SourceFingerprint,
		CreatedAt:          s.CreatedAt,
		UpdatedAt:          s.UpdatedAt,
	}
}

func FromDomainService(d domain.Service) Service {
	return Service{
		ID:                 d.ID,
		Name:               d.Name,
		ServerID:           d.ServerID,
		Type:               string(d.Type),
		ApplicationID:      d.ApplicationID,
		WorkDir:            d.WorkDir,
		ExposedPort:        d.ExposedPort,
		WebhookSecret:      d.WebhookSecret,
		CurrentReleaseID:   d.CurrentReleaseID,
		AutoRollbackOnFail: d.AutoRollbackOnFail,
		AutoSync:           d.AutoSync,
		SyncInterval:       d.SyncInterval,
		SyncStatus:         d.SyncStatus,
		SourceKind:         d.SourceKind,
		SourceID:           d.SourceID,
		SourceFingerprint:  d.SourceFingerprint,
		CreatedAt:          d.CreatedAt,
		UpdatedAt:          d.UpdatedAt,
	}
}

// ── Application ────────────────────────────────────────────────────────

func ToDomainApplication(a Application) domain.Application {
	return domain.Application{
		ID:               a.ID,
		Name:             a.Name,
		Description:      a.Description,
		ServerID:         a.ServerID,
		RunServerID:      a.RunServerID,
		PrimaryServiceID: a.PrimaryServiceID,
		SiteName:         a.SiteName,
		Domain:           a.Domain,
		ContainerName:    a.ContainerName,
		BaseDir:          a.BaseDir,
		ExposeMode:       a.ExposeMode,
		DeployID:         a.DeployID,
		DBConnID:         a.DBConnID,
		CreatedAt:        a.CreatedAt,
		UpdatedAt:        a.UpdatedAt,
	}
}

func FromDomainApplication(d domain.Application) Application {
	return Application{
		ID:               d.ID,
		Name:             d.Name,
		Description:      d.Description,
		ServerID:         d.ServerID,
		RunServerID:      d.RunServerID,
		PrimaryServiceID: d.PrimaryServiceID,
		SiteName:         d.SiteName,
		Domain:           d.Domain,
		ContainerName:    d.ContainerName,
		BaseDir:          d.BaseDir,
		ExposeMode:       d.ExposeMode,
		DeployID:         d.DeployID,
		DBConnID:         d.DBConnID,
		CreatedAt:        d.CreatedAt,
		UpdatedAt:        d.UpdatedAt,
	}
}

// ── Ingress ────────────────────────────────────────────────────────────

func ToDomainIngress(i Ingress) domain.Ingress {
	return domain.Ingress{
		ID:                 i.ID,
		EdgeServerID:       i.EdgeServerID,
		MatchKind:          i.MatchKind,
		Domain:             i.Domain,
		DefaultPath:        i.DefaultPath,
		CertID:             i.CertID,
		ForceHTTPS:         i.ForceHTTPS,
		Status:             i.Status,
		LastAppliedAt:      i.LastAppliedAt,
		ArchivePath:        i.ArchivePath,
		OriginalConfigPath: i.OriginalConfigPath,
		CreatedAt:          i.CreatedAt,
		UpdatedAt:          i.UpdatedAt,
	}
}

func FromDomainIngress(d domain.Ingress) Ingress {
	return Ingress{
		ID:                 d.ID,
		EdgeServerID:       d.EdgeServerID,
		MatchKind:          d.MatchKind,
		Domain:             d.Domain,
		DefaultPath:        d.DefaultPath,
		CertID:             d.CertID,
		ForceHTTPS:         d.ForceHTTPS,
		Status:             d.Status,
		LastAppliedAt:      d.LastAppliedAt,
		ArchivePath:        d.ArchivePath,
		OriginalConfigPath: d.OriginalConfigPath,
		CreatedAt:          d.CreatedAt,
		UpdatedAt:          d.UpdatedAt,
	}
}

// ── IngressRoute / IngressUpstream ─────────────────────────────────────

func ToDomainIngressRoute(r IngressRoute) domain.IngressRoute {
	return domain.IngressRoute{
		ID:        r.ID,
		IngressID: r.IngressID,
		Sort:      r.Sort,
		Path:      r.Path,
		Protocol:  r.Protocol,
		Upstream: domain.IngressUpstream{
			Type:         r.Upstream.Type,
			ServiceID:    r.Upstream.ServiceID,
			RawURL:       r.Upstream.RawURL,
			NetworkPref:  r.Upstream.NetworkPref,
			OverrideHost: r.Upstream.OverrideHost,
			OverridePort: r.Upstream.OverridePort,
		},
		WebSocket:  r.WebSocket,
		Extra:      r.Extra,
		ListenPort: r.ListenPort,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
}

func FromDomainIngressRoute(d domain.IngressRoute) IngressRoute {
	return IngressRoute{
		ID:        d.ID,
		IngressID: d.IngressID,
		Sort:      d.Sort,
		Path:      d.Path,
		Protocol:  d.Protocol,
		Upstream: IngressUpstream{
			Type:         d.Upstream.Type,
			ServiceID:    d.Upstream.ServiceID,
			RawURL:       d.Upstream.RawURL,
			NetworkPref:  d.Upstream.NetworkPref,
			OverrideHost: d.Upstream.OverrideHost,
			OverridePort: d.Upstream.OverridePort,
		},
		WebSocket:  d.WebSocket,
		Extra:      d.Extra,
		ListenPort: d.ListenPort,
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
	}
}

// ── Release ─────────────────────────────────────────────────────────────

func ToDomainRelease(r Release) domain.Release {
	return domain.Release{
		ID:          r.ID,
		ServiceID:   r.ServiceID,
		Label:       r.Label,
		Version:     r.Label,
		ArtifactID:  r.ArtifactID,
		EnvSetID:    r.EnvSetID,
		ConfigSetID: r.ConfigSetID,
		StartSpec:   r.StartSpec,
		Note:        r.Note,
		CreatedBy:   r.CreatedBy,
		Status:      r.Status,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func FromDomainRelease(d domain.Release) Release {
	return Release{
		ID:          d.ID,
		ServiceID:   d.ServiceID,
		Label:       d.Label,
		ArtifactID:  d.ArtifactID,
		EnvSetID:    d.EnvSetID,
		ConfigSetID: d.ConfigSetID,
		StartSpec:   d.StartSpec,
		Note:        d.Note,
		CreatedBy:   d.CreatedBy,
		Status:      d.Status,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

// ── Artifact ────────────────────────────────────────────────────────────

func ToDomainArtifact(a Artifact) domain.Artifact {
	return domain.Artifact{
		ID:         a.ID,
		ServiceID:  a.ServiceID,
		Provider:   a.Provider,
		Ref:        a.Ref,
		PullScript: a.PullScript,
		Checksum:   a.Checksum,
		SizeBytes:  a.SizeBytes,
		CreatedAt:  a.CreatedAt,
	}
}

func FromDomainArtifact(d domain.Artifact) Artifact {
	return Artifact{
		ID:         d.ID,
		ServiceID:  d.ServiceID,
		Provider:   d.Provider,
		Ref:        d.Ref,
		PullScript: d.PullScript,
		Checksum:   d.Checksum,
		SizeBytes:  d.SizeBytes,
		CreatedAt:  d.CreatedAt,
	}
}

// ── EnvVarSet ──────────────────────────────────────────────────────────

func ToDomainEnvVarSet(e EnvVarSet) domain.EnvVarSet {
	return domain.EnvVarSet{
		ID:        e.ID,
		ServiceID: e.ServiceID,
		Label:     e.Label,
		Content:   e.Content,
		CreatedAt: e.CreatedAt,
	}
}

func FromDomainEnvVarSet(d domain.EnvVarSet) EnvVarSet {
	return EnvVarSet{
		ID:        d.ID,
		ServiceID: d.ServiceID,
		Label:     d.Label,
		Content:   d.Content,
		CreatedAt: d.CreatedAt,
	}
}

// ── ConfigFileSet ──────────────────────────────────────────────────────

func ToDomainConfigFileSet(c ConfigFileSet) domain.ConfigFileSet {
	return domain.ConfigFileSet{
		ID:        c.ID,
		ServiceID: c.ServiceID,
		Label:     c.Label,
		Files:     c.Files,
		CreatedAt: c.CreatedAt,
	}
}

func FromDomainConfigFileSet(d domain.ConfigFileSet) ConfigFileSet {
	return ConfigFileSet{
		ID:        d.ID,
		ServiceID: d.ServiceID,
		Label:     d.Label,
		Files:     d.Files,
		CreatedAt: d.CreatedAt,
	}
}

// ── User ────────────────────────────────────────────────────────────────

func ToDomainUser(u User) domain.User {
	return domain.User{
		ID:           u.ID,
		Username:     u.Username,
		Password:     u.Password,
		Role:         u.Role,
		MFASecret:    u.MFASecret,
		MFAEnabled:   u.MFAEnabled,
		LastTOTPStep: u.LastTOTPStep,
		LastLogin:    u.LastLogin,
		LastIP:       u.LastIP,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}

func FromDomainUser(d domain.User) User {
	return User{
		ID:           d.ID,
		Username:     d.Username,
		Password:     d.Password,
		Role:         d.Role,
		MFASecret:    d.MFASecret,
		MFAEnabled:   d.MFAEnabled,
		LastTOTPStep: d.LastTOTPStep,
		LastLogin:    d.LastLogin,
		LastIP:       d.LastIP,
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
	}
}

// ── DeployRun ──────────────────────────────────────────────────────────

func ToDomainDeployRun(d DeployRun) domain.DeployRun {
	return domain.DeployRun{
		ID:                d.ID,
		ServiceID:         d.ServiceID,
		ReleaseID:         d.ReleaseID,
		Status:            d.Status,
		TriggerSource:     d.TriggerSource,
		StartedAt:         d.StartedAt,
		FinishedAt:        d.FinishedAt,
		DurationSec:       d.DurationSec,
		Output:            d.Output,
		RollbackFromRunID: d.RollbackFromRunID,
		CreatedAt:         d.CreatedAt,
	}
}

func FromDomainDeployRun(d domain.DeployRun) DeployRun {
	return DeployRun{
		ID:                d.ID,
		ServiceID:         d.ServiceID,
		ReleaseID:         d.ReleaseID,
		Status:            d.Status,
		TriggerSource:     d.TriggerSource,
		StartedAt:         d.StartedAt,
		FinishedAt:        d.FinishedAt,
		DurationSec:       d.DurationSec,
		Output:            d.Output,
		RollbackFromRunID: d.RollbackFromRunID,
		CreatedAt:         d.CreatedAt,
	}
}

// ── DeployLog ──────────────────────────────────────────────────────────

func ToDomainDeployLog(d DeployLog) domain.DeployLog {
	return domain.DeployLog{
		ID:            d.ID,
		DeployID:      d.DeployID,
		Output:        d.Output,
		Status:        d.Status,
		Duration:      d.Duration,
		TriggerSource: d.TriggerSource,
		CreatedAt:     d.CreatedAt,
	}
}

func FromDomainDeployLog(d domain.DeployLog) DeployLog {
	return DeployLog{
		ID:            d.ID,
		DeployID:      d.DeployID,
		Output:        d.Output,
		Status:        d.Status,
		Duration:      d.Duration,
		TriggerSource: d.TriggerSource,
		CreatedAt:     d.CreatedAt,
	}
}

// ── AppReleaseSet ──────────────────────────────────────────────────────

func ToDomainAppReleaseSet(a AppReleaseSet) domain.AppReleaseSet {
	return domain.AppReleaseSet{
		ID:            a.ID,
		ApplicationID: a.ApplicationID,
		Label:         a.Label,
		Items:         a.Items,
		Note:          a.Note,
		Status:        a.Status,
		CreatedBy:     a.CreatedBy,
		AppliedAt:     a.AppliedAt,
		LastSummary:   a.LastSummary,
		CreatedAt:     a.CreatedAt,
		UpdatedAt:     a.UpdatedAt,
	}
}

func FromDomainAppReleaseSet(d domain.AppReleaseSet) AppReleaseSet {
	return AppReleaseSet{
		ID:            d.ID,
		ApplicationID: d.ApplicationID,
		Label:         d.Label,
		Items:         d.Items,
		Note:          d.Note,
		Status:        d.Status,
		CreatedBy:     d.CreatedBy,
		AppliedAt:     d.AppliedAt,
		LastSummary:   d.LastSummary,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}
}

// ── ServerProbe ────────────────────────────────────────────────────────

func ToDomainServerProbe(p ServerProbe) domain.ServerProbe {
	return domain.ServerProbe{
		ID:        p.ID,
		ServerID:  p.ServerID,
		Result:    p.Result,
		LatencyMs: p.LatencyMs,
		ErrMsg:    p.ErrMsg,
		CreatedAt: p.CreatedAt,
	}
}

func FromDomainServerProbe(d domain.ServerProbe) ServerProbe {
	return ServerProbe{
		ID:        d.ID,
		ServerID:  d.ServerID,
		Result:    d.Result,
		LatencyMs: d.LatencyMs,
		ErrMsg:    d.ErrMsg,
		CreatedAt: d.CreatedAt,
	}
}

// ── DBConn ─────────────────────────────────────────────────────────────

func ToDomainDBConn(c DBConn) domain.DBConn {
	var deletedAt *time.Time
	if c.DeletedAt.Valid {
		deletedAt = &c.DeletedAt.Time
	}
	return domain.DBConn{
		ID:            c.ID,
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
		DeletedAt:     deletedAt,
		ServerID:      c.ServerID,
		ApplicationID: c.ApplicationID,
		Name:          c.Name,
		Type:          c.Type,
		Host:          c.Host,
		Port:          c.Port,
		Username:      c.Username,
		Password:      c.Password,
		Database:      c.Database,
	}
}

func FromDomainDBConn(d domain.DBConn) DBConn {
	c := DBConn{
		Model: gorm.Model{
			ID:        d.ID,
			CreatedAt: d.CreatedAt,
			UpdatedAt: d.UpdatedAt,
		},
		ServerID:      d.ServerID,
		ApplicationID: d.ApplicationID,
		Name:          d.Name,
		Type:          d.Type,
		Host:          d.Host,
		Port:          d.Port,
		Username:      d.Username,
		Password:      d.Password,
		Database:      d.Database,
	}
	if d.DeletedAt != nil {
		c.DeletedAt = gorm.DeletedAt{Time: *d.DeletedAt, Valid: true}
	}
	return c
}

// ── SSLCert ────────────────────────────────────────────────────────────

func ToDomainSSLCert(s SSLCert) domain.SSLCert {
	var deletedAt *time.Time
	if s.DeletedAt.Valid {
		deletedAt = &s.DeletedAt.Time
	}
	return domain.SSLCert{
		ID:            s.ID,
		CreatedAt:     s.CreatedAt,
		UpdatedAt:     s.UpdatedAt,
		DeletedAt:     deletedAt,
		ServerID:      s.ServerID,
		ApplicationID: s.ApplicationID,
		Domain:        s.Domain,
		CertPath:      s.CertPath,
		KeyPath:       s.KeyPath,
		Issuer:        s.Issuer,
		ExpiresAt:     s.ExpiresAt,
		AutoRenew:     s.AutoRenew,
		CertPEM:       s.CertPEM,
		KeyPEM:        s.KeyPEM,
		LastRenewedAt: s.LastRenewedAt,
	}
}

func FromDomainSSLCert(d domain.SSLCert) SSLCert {
	c := SSLCert{
		Model: gorm.Model{
			ID:        d.ID,
			CreatedAt: d.CreatedAt,
			UpdatedAt: d.UpdatedAt,
		},
		ServerID:      d.ServerID,
		ApplicationID: d.ApplicationID,
		Domain:        d.Domain,
		CertPath:      d.CertPath,
		KeyPath:       d.KeyPath,
		Issuer:        d.Issuer,
		ExpiresAt:     d.ExpiresAt,
		AutoRenew:     d.AutoRenew,
		CertPEM:       d.CertPEM,
		KeyPEM:        d.KeyPEM,
		LastRenewedAt: d.LastRenewedAt,
	}
	if d.DeletedAt != nil {
		c.DeletedAt = gorm.DeletedAt{Time: *d.DeletedAt, Valid: true}
	}
	return c
}

// ── NginxProfile ────────────────────────────────────────────────────────

func ToDomainNginxProfile(p NginxProfile) domain.NginxProfile {
	var deletedAt *time.Time
	if p.DeletedAt.Valid {
		deletedAt = &p.DeletedAt.Time
	}
	return domain.NginxProfile{
		ID:                p.ID,
		CreatedAt:         p.CreatedAt,
		UpdatedAt:         p.UpdatedAt,
		DeletedAt:         deletedAt,
		EdgeServerID:      p.EdgeServerID,
		NginxConfDir:      p.NginxConfDir,
		SitesAvailableDir: p.SitesAvailableDir,
		SitesEnabledDir:   p.SitesEnabledDir,
		AppLocationsDir:   p.AppLocationsDir,
		StreamsConf:       p.StreamsConf,
		CertDir:           p.CertDir,
		NginxConfPath:     p.NginxConfPath,
		HubSiteName:       p.HubSiteName,
		TestCmd:           p.TestCmd,
		ReloadCmd:         p.ReloadCmd,
		BinaryPath:        p.BinaryPath,
		NginxVRaw:         p.NginxVRaw,
		Version:           p.Version,
		BuildPrefix:       p.BuildPrefix,
		BuildConf:         p.BuildConf,
		Modules:           p.Modules,
		LastProbeAt:       p.LastProbeAt,
	}
}

func FromDomainNginxProfile(d domain.NginxProfile) NginxProfile {
	p := NginxProfile{
		Model: gorm.Model{
			ID:        d.ID,
			CreatedAt: d.CreatedAt,
			UpdatedAt: d.UpdatedAt,
		},
		EdgeServerID:      d.EdgeServerID,
		NginxConfDir:      d.NginxConfDir,
		SitesAvailableDir: d.SitesAvailableDir,
		SitesEnabledDir:   d.SitesEnabledDir,
		AppLocationsDir:   d.AppLocationsDir,
		StreamsConf:       d.StreamsConf,
		CertDir:           d.CertDir,
		NginxConfPath:     d.NginxConfPath,
		HubSiteName:       d.HubSiteName,
		TestCmd:           d.TestCmd,
		ReloadCmd:         d.ReloadCmd,
		BinaryPath:        d.BinaryPath,
		NginxVRaw:         d.NginxVRaw,
		Version:           d.Version,
		BuildPrefix:       d.BuildPrefix,
		BuildConf:         d.BuildConf,
		Modules:           d.Modules,
		LastProbeAt:       d.LastProbeAt,
	}
	if d.DeletedAt != nil {
		p.DeletedAt = gorm.DeletedAt{Time: *d.DeletedAt, Valid: true}
	}
	return p
}

// ── NginxCert ───────────────────────────────────────────────────────────

func ToDomainNginxCert(c NginxCert) domain.NginxCert {
	return domain.NginxCert{
		ID:        c.ID,
		Domain:    c.Domain,
		Source:    c.Source,
		CertPEM:   c.CertPEM,
		KeyPEM:    c.KeyPEM,
		ExpiresAt: c.ExpiresAt,
		AutoRenew: c.AutoRenew,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

func FromDomainNginxCert(d domain.NginxCert) NginxCert {
	return NginxCert{
		ID:        d.ID,
		Domain:    d.Domain,
		Source:    d.Source,
		CertPEM:   d.CertPEM,
		KeyPEM:    d.KeyPEM,
		ExpiresAt: d.ExpiresAt,
		AutoRenew: d.AutoRenew,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

// ── Settings ────────────────────────────────────────────────────────────

func ToDomainSetting(s Setting) domain.Setting {
	return domain.Setting{
		Key:       s.Key,
		Value:     s.Value,
		UpdatedAt: s.UpdatedAt,
	}
}

func FromDomainSetting(d domain.Setting) Setting {
	return Setting{
		Key:       d.Key,
		Value:     d.Value,
		UpdatedAt: d.UpdatedAt,
	}
}

// ── Alert ───────────────────────────────────────────────────────────────

func ToDomainAlertRule(r AlertRule) domain.AlertRule {
	return domain.AlertRule{
		ID:        r.ID,
		ServerID:  r.ServerID,
		Metric:    r.Metric,
		Operator:  r.Operator,
		Threshold: r.Threshold,
		Duration:  r.Duration,
		Enabled:   r.Enabled,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

func FromDomainAlertRule(d domain.AlertRule) AlertRule {
	return AlertRule{
		ID:        d.ID,
		ServerID:  d.ServerID,
		Metric:    d.Metric,
		Operator:  d.Operator,
		Threshold: d.Threshold,
		Duration:  d.Duration,
		Enabled:   d.Enabled,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

func ToDomainAlertEvent(e AlertEvent) domain.AlertEvent {
	return domain.AlertEvent{
		ID:       e.ID,
		RuleID:   e.RuleID,
		ServerID: e.ServerID,
		Value:    e.Value,
		Message:  e.Message,
		SentAt:   e.SentAt,
	}
}

func FromDomainAlertEvent(d domain.AlertEvent) AlertEvent {
	return AlertEvent{
		ID:       d.ID,
		RuleID:   d.RuleID,
		ServerID: d.ServerID,
		Value:    d.Value,
		Message:  d.Message,
		SentAt:   d.SentAt,
	}
}

func ToDomainNotifyChannel(c NotifyChannel) domain.NotifyChannel {
	return domain.NotifyChannel{
		ID:        c.ID,
		Name:      c.Name,
		Type:      c.Type,
		URL:       c.URL,
		Template:  c.Template,
		Enabled:   c.Enabled,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

func FromDomainNotifyChannel(d domain.NotifyChannel) NotifyChannel {
	return NotifyChannel{
		ID:        d.ID,
		Name:      d.Name,
		Type:      d.Type,
		URL:       d.URL,
		Template:  d.Template,
		Enabled:   d.Enabled,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

// ── Audit ───────────────────────────────────────────────────────────────

func ToDomainAuditApply(a AuditApply) domain.AuditApply {
	return domain.AuditApply{
		ID:            a.ID,
		EdgeServerID:  a.EdgeServerID,
		ActorUserID:   a.ActorUserID,
		ChangesetDiff: a.ChangesetDiff,
		NginxTOutput:  a.NginxTOutput,
		RolledBack:    a.RolledBack,
		BackupPath:    a.BackupPath,
		DurationMs:    a.DurationMs,
		CreatedAt:     a.CreatedAt,
	}
}

func FromDomainAuditApply(d domain.AuditApply) AuditApply {
	return AuditApply{
		ID:            d.ID,
		EdgeServerID:  d.EdgeServerID,
		ActorUserID:   d.ActorUserID,
		ChangesetDiff: d.ChangesetDiff,
		NginxTOutput:  d.NginxTOutput,
		RolledBack:    d.RolledBack,
		BackupPath:    d.BackupPath,
		DurationMs:    d.DurationMs,
		CreatedAt:     d.CreatedAt,
	}
}

func ToDomainAuditLog(l AuditLog) domain.AuditLog {
	return domain.AuditLog{
		ID:         l.ID,
		UserID:     l.UserID,
		Username:   l.Username,
		IP:         l.IP,
		Method:     l.Method,
		Path:       l.Path,
		Body:       l.Body,
		Status:     l.Status,
		DurationMS: l.DurationMS,
		CreatedAt:  l.CreatedAt,
	}
}

func FromDomainAuditLog(d domain.AuditLog) AuditLog {
	return AuditLog{
		ID:         d.ID,
		UserID:     d.UserID,
		Username:   d.Username,
		IP:         d.IP,
		Method:     d.Method,
		Path:       d.Path,
		Body:       d.Body,
		Status:     d.Status,
		DurationMS: d.DurationMS,
		CreatedAt:  d.CreatedAt,
	}
}

// ── Metric ──────────────────────────────────────────────────────────────

func ToDomainMetric(m Metric) domain.Metric {
	return domain.Metric{
		ID:        m.ID,
		ServerID:  m.ServerID,
		CPU:       m.CPU,
		Mem:       m.Mem,
		Disk:      m.Disk,
		Load1:     m.Load1,
		Uptime:    m.Uptime,
		CreatedAt: m.CreatedAt,
	}
}

func FromDomainMetric(d domain.Metric) Metric {
	return Metric{
		ID:        d.ID,
		ServerID:  d.ServerID,
		CPU:       d.CPU,
		Mem:       d.Mem,
		Disk:      d.Disk,
		Load1:     d.Load1,
		Uptime:    d.Uptime,
		CreatedAt: d.CreatedAt,
	}
}

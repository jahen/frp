package config

import (
	"github.com/fatedier/frp/pkg/config/legacy"
	v1 "github.com/fatedier/frp/pkg/config/v1"
)

// LoadServerConfigFromContent loads server config from raw content (string).
// It mirrors the behavior of LoadServerConfig but works on in-memory content
// instead of reading from a file.
func LoadServerConfigFromContent(content string, strict bool) (*v1.ServerConfig, bool, error) {
	var (
		svrCfg         *v1.ServerConfig
		isLegacyFormat bool
	)
	// detect legacy ini format
	if DetectLegacyINIFormat([]byte(content)) {
		legacyCfg, err := legacy.UnmarshalServerConfFromIni([]byte(content))
		if err != nil {
			return nil, true, err
		}
		svrCfg = legacy.Convert_ServerCommonConf_To_v1(&legacyCfg)
		isLegacyFormat = true
	} else {
		svrCfg = &v1.ServerConfig{}
		con, err := RenderWithTemplate([]byte(content), GetValues())
		if err != nil {
			return nil, false, err
		}
		if err := LoadConfigure(con, svrCfg, strict); err != nil {
			return nil, false, err
		}
	}
	if svrCfg != nil {
		if err := svrCfg.Complete(); err != nil {
			return nil, isLegacyFormat, err
		}
	}
	return svrCfg, isLegacyFormat, nil
}

// LoadClientConfigFromContent loads client config from raw content (string).
// It mirrors the behavior of LoadClientConfig but works on in-memory content
// instead of reading from a file.
func LoadClientConfigFromContent(content string, strict bool) (
	*v1.ClientCommonConfig,
	[]v1.ProxyConfigurer,
	[]v1.VisitorConfigurer,
	bool, error,
) {
	var (
		cliCfg         *v1.ClientCommonConfig
		proxyCfgs      = make([]v1.ProxyConfigurer, 0)
		visitorCfgs    = make([]v1.VisitorConfigurer, 0)
		isLegacyFormat bool
	)

	if DetectLegacyINIFormat([]byte(content)) {
		legacyCommon, legacyProxyCfgs, legacyVisitorCfgs, err := legacy.ParseClientConfigFromContent(content)
		if err != nil {
			return nil, nil, nil, true, err
		}
		cliCfg = legacy.Convert_ClientCommonConf_To_v1(&legacyCommon)
		for _, c := range legacyProxyCfgs {
			proxyCfgs = append(proxyCfgs, legacy.Convert_ProxyConf_To_v1(c))
		}
		for _, c := range legacyVisitorCfgs {
			visitorCfgs = append(visitorCfgs, legacy.Convert_VisitorConf_To_v1(c))
		}
		isLegacyFormat = true
	} else {
		allCfg := v1.ClientConfig{}
		con, err := RenderWithTemplate([]byte(content), GetValues())
		if err != nil {
			return nil, nil, nil, false, err
		}
		if err := LoadConfigure(con, &allCfg, strict); err != nil {
			return nil, nil, nil, false, err
		}
		cliCfg = &allCfg.ClientCommonConfig
		for _, c := range allCfg.Proxies {
			proxyCfgs = append(proxyCfgs, c.ProxyConfigurer)
		}
		for _, c := range allCfg.Visitors {
			visitorCfgs = append(visitorCfgs, c.VisitorConfigurer)
		}
	}

	// Load additional config from includes.
	// legacy ini format already handle this in ParseClientConfig.
	if len(cliCfg.IncludeConfigFiles) > 0 && !isLegacyFormat {
		extProxyCfgs, extVisitorCfgs, err := LoadAdditionalClientConfigs(cliCfg.IncludeConfigFiles, isLegacyFormat, strict)
		if err != nil {
			return nil, nil, nil, isLegacyFormat, err
		}
		proxyCfgs = append(proxyCfgs, extProxyCfgs...)
		visitorCfgs = append(visitorCfgs, extVisitorCfgs...)
	}

	// Complete the common config first (required before filtering)
	if cliCfg != nil {
		if err := cliCfg.Complete(); err != nil {
			return nil, nil, nil, isLegacyFormat, err
		}
	}

	proxyCfgs, visitorCfgs = FilterClientConfigurers(cliCfg, proxyCfgs, visitorCfgs)
	proxyCfgs = CompleteProxyConfigurers(proxyCfgs)
	visitorCfgs = CompleteVisitorConfigurers(visitorCfgs)
	return cliCfg, proxyCfgs, visitorCfgs, isLegacyFormat, nil
}

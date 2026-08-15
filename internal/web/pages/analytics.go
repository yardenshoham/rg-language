package pages

import (
	"fmt"

	g "maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// Analytics is the optional PostHog configuration. The zero value renders no
// script at all, which is the default everywhere: the site is complete without
// it, and the tests and the browser suite run with it off.
type Analytics struct {
	// PostHogKey is the project API key. Empty means no analytics.
	PostHogKey string
	// PostHogHost is the ingestion host, defaulting to PostHog's EU cloud.
	PostHogHost string
	// PostHogUIHost is where the dashboard lives. It only matters when
	// PostHogHost is a reverse proxy, so that links point back at PostHog.
	PostHogUIHost string
}

// defaultPostHogHost is PostHog's EU cloud, which is where this project's
// project lives; a reverse proxy replaces it.
const defaultPostHogHost = "https://eu.i.posthog.com"

// posthogLoader is PostHog's own snippet, copied verbatim from the install page.
// It stubs the API, queues calls made before the real library lands, and pulls
// array.js from the assets host. Do not tidy it — it is minified upstream code,
// and the only thing this project adds is the init call below.
const posthogLoader = `!function(t,e){var o,n,p,r;e.__SV||(window.posthog && window.posthog.__loaded)||(window.posthog=e,e._i=[],e.init=function(i,s,a){function g(t,e){var o=e.split(".");2==o.length&&(t=t[o[0]],e=o[1]),t[e]=function(){t.push([e].concat(Array.prototype.slice.call(arguments,0)))}}p||((p=t.createElement("script")).type="text/javascript",p.crossOrigin="anonymous",p.async=!0,p.src=s.api_host.replace(".i.posthog.com","-assets.i.posthog.com")+"/static/array.js",p.onerror=function(){p=null},(r=t.getElementsByTagName("script")[0]).parentNode.insertBefore(p,r));var u=e;for(void 0!==a?u=e[a]=[]:a="posthog",u.people=u.people||[],u.toString=function(t){var e="posthog";return"posthog"!==a&&(e+="."+a),t||(e+=" (stub)"),e},u.people.toString=function(){return u.toString(1)+".people (stub)"},o="Sn Cn init Hn Un Gn Yi zn Kn qn capture Vn kn calculateEventProperties es register register_once register_for_session unregister unregister_for_session os Bn ss getFeatureFlag getFeatureFlagPayload getFeatureFlagResult getAllFeatureFlags isFeatureEnabled reloadFeatureFlags updateFlags updateEarlyAccessFeatureEnrollment getEarlyAccessFeatures on onFeatureFlags onSurveysLoaded onSessionId getSurveys getActiveMatchingSurveys renderSurvey displaySurvey cancelPendingSurvey canRenderSurvey canRenderSurveyAsync ls identify setPersonProperties unsetPersonProperties group resetGroups setPersonPropertiesForFlags resetPersonPropertiesForFlags setGroupPropertiesForFlags resetGroupPropertiesForFlags reset us shutdown setIdentity clearIdentity get_distinct_id getGroups get_session_id get_session_replay_url alias set_config startSessionRecording stopSessionRecording sessionRecordingStarted captureException addExceptionStep captureLog startExceptionAutocapture stopExceptionAutocapture loadToolbar get_property getSessionProperty rs Xn createPersonProfile setInternalOrTestUser ns $n vs opt_in_capturing opt_out_capturing has_opted_in_capturing has_opted_out_capturing get_explicit_consent_status is_capturing clear_opt_in_out_capturing Jn debug tr At getPageViewId captureTraceFeedback captureTraceMetric Ln".split(" "),n=0;n<o.length;n++)g(u,o[n]);e._i.push([i,s,a])},e.__SV=1)}(document,window.posthog||[]);`

// posthogScript renders the loader and the init call, or nothing when no key is
// configured. %q does the quoting, so a key or host from the environment cannot
// break out of the JavaScript string it lands in.
func posthogScript(analytics Analytics) g.Node {
	if analytics.PostHogKey == "" {
		return nil
	}
	host := analytics.PostHogHost
	if host == "" {
		host = defaultPostHogHost
	}
	options := fmt.Sprintf("api_host:%q", host)
	if analytics.PostHogUIHost != "" {
		options += fmt.Sprintf(",ui_host:%q", analytics.PostHogUIHost)
	}
	return html.Script(g.Raw(fmt.Sprintf("%s\nposthog.init(%q,{%s,defaults:'2026-05-30',person_profiles:'identified_only'})",
		posthogLoader, analytics.PostHogKey, options)))
}

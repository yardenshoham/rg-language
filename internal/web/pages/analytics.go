package pages

import (
	"cmp"
	"fmt"

	g "maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// Analytics is the optional PostHog configuration; the zero value renders no script.
type Analytics struct {
	PostHogKey    string // project API key; empty means no analytics
	PostHogHost   string // ingestion host, defaulting to PostHog's EU cloud
	PostHogUIHost string // dashboard host, so links point back when ingestion is a proxy
}

const defaultPostHogHost = "https://eu.i.posthog.com"

// posthogLoader is PostHog's own minified snippet, verbatim from their install page — do not tidy it.
const posthogLoader = `!function(t,e){var o,n,p,r;e.__SV||(window.posthog && window.posthog.__loaded)||(window.posthog=e,e._i=[],e.init=function(i,s,a){function g(t,e){var o=e.split(".");2==o.length&&(t=t[o[0]],e=o[1]),t[e]=function(){t.push([e].concat(Array.prototype.slice.call(arguments,0)))}}p||((p=t.createElement("script")).type="text/javascript",p.crossOrigin="anonymous",p.async=!0,p.src=s.api_host.replace(".i.posthog.com","-assets.i.posthog.com")+"/static/array.js",p.onerror=function(){p=null},(r=t.getElementsByTagName("script")[0]).parentNode.insertBefore(p,r));var u=e;for(void 0!==a?u=e[a]=[]:a="posthog",u.people=u.people||[],u.toString=function(t){var e="posthog";return"posthog"!==a&&(e+="."+a),t||(e+=" (stub)"),e},u.people.toString=function(){return u.toString(1)+".people (stub)"},o="Sn Cn init Hn Un Gn Yi zn Kn qn capture Vn kn calculateEventProperties es register register_once register_for_session unregister unregister_for_session os Bn ss getFeatureFlag getFeatureFlagPayload getFeatureFlagResult getAllFeatureFlags isFeatureEnabled reloadFeatureFlags updateFlags updateEarlyAccessFeatureEnrollment getEarlyAccessFeatures on onFeatureFlags onSurveysLoaded onSessionId getSurveys getActiveMatchingSurveys renderSurvey displaySurvey cancelPendingSurvey canRenderSurvey canRenderSurveyAsync ls identify setPersonProperties unsetPersonProperties group resetGroups setPersonPropertiesForFlags resetPersonPropertiesForFlags setGroupPropertiesForFlags resetGroupPropertiesForFlags reset us shutdown setIdentity clearIdentity get_distinct_id getGroups get_session_id get_session_replay_url alias set_config startSessionRecording stopSessionRecording sessionRecordingStarted captureException addExceptionStep captureLog startExceptionAutocapture stopExceptionAutocapture loadToolbar get_property getSessionProperty rs Xn createPersonProfile setInternalOrTestUser ns $n vs opt_in_capturing opt_out_capturing has_opted_in_capturing has_opted_out_capturing get_explicit_consent_status is_capturing clear_opt_in_out_capturing Jn debug tr At getPageViewId captureTraceFeedback captureTraceMetric Ln".split(" "),n=0;n<o.length;n++)g(u,o[n]);e._i.push([i,s,a])},e.__SV=1)}(document,window.posthog||[]);`

// posthogScript renders nothing without a key; %q keeps a key or host inside the JS string.
func posthogScript(analytics Analytics) g.Node {
	if analytics.PostHogKey == "" {
		return nil
	}
	options := fmt.Sprintf("api_host:%q", cmp.Or(analytics.PostHogHost, defaultPostHogHost))
	if analytics.PostHogUIHost != "" {
		options += fmt.Sprintf(",ui_host:%q", analytics.PostHogUIHost)
	}
	return html.Script(g.Raw(fmt.Sprintf("%s\nposthog.init(%q,{%s,defaults:'2026-05-30',person_profiles:'identified_only'})",
		posthogLoader, analytics.PostHogKey, options)))
}

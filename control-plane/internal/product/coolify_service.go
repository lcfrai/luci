package product

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Reuse the image pinned and licensed with the native Supabase distribution.
const dataIngressImage = "envoyproxy/envoy:v1.39.1@sha256:57e14a549d7bd43c8d3f6d03e8cfa653e037d4b38e133acd9b54f38c524401b4"

type coolifyService struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Compose     string `json:"docker_compose_raw"`
	Destination struct {
		Server struct {
			UUID string `json:"uuid"`
		} `json:"server"`
	} `json:"destination"`
	Applications []struct {
		Name string `json:"name"`
		FQDN string `json:"fqdn"`
	} `json:"applications"`
}

func (c *CoolifyClient) dataServices(ctx context.Context) ([]coolifyService, error) {
	var list []coolifyService
	if err := c.request(ctx, http.MethodGet, "/services", nil, nil, &list); err != nil {
		return nil, err
	}
	if len(list) > 2000 {
		return nil, errors.New("Coolify returned too many service records")
	}
	return list, nil
}
func (c *CoolifyClient) dataService(ctx context.Context, id string) (coolifyService, error) {
	var v coolifyService
	if !coolifyID.MatchString(id) {
		return v, errors.New("invalid owned data service identifier")
	}
	err := c.request(ctx, http.MethodGet, "/services/"+id, nil, nil, &v)
	if err == nil && v.UUID != id {
		err = errors.New("Coolify returned a different data service")
	}
	return v, err
}
func dataServiceName(v dataDomainRecord) string   { return "luci-data-" + v.Owner }
func dataServiceMarker(v dataDomainRecord) string { return "Luci public data ingress " + v.Owner }
func dataServiceURL(v dataDomainRecord) string    { return v.Domain + ":8080" }

func dataComposeHash(compose string) (string, error) {
	if len(compose) > 128<<10 {
		return "", errors.New("data ingress configuration exceeds its limit")
	}
	var parsed map[string]any
	if yaml.Unmarshal([]byte(compose), &parsed) != nil || len(parsed) == 0 {
		return "", errors.New("data ingress configuration could not be confirmed")
	}
	b, err := json.Marshal(parsed)
	if err != nil {
		return "", errors.New("data ingress configuration could not be confirmed")
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}
func ownedDataServiceConfiguration(v dataDomainRecord, observed coolifyService, allowPrevious bool) error {
	if !coolifyID.MatchString(observed.UUID) || (v.ProviderServiceUUID != "" && observed.UUID != v.ProviderServiceUUID) || observed.Name != dataServiceName(v) || observed.Description != dataServiceMarker(v) || observed.Destination.Server.UUID != v.ServerUUID {
		return errors.New("the provider service no longer matches this project's ownership; no mutation was made")
	}
	hash, err := dataComposeHash(observed.Compose)
	if err != nil {
		return errors.New("grant Coolify read:sensitive access to confirm this owned ingress configuration")
	}
	if hash != v.ComposeHash && !(allowPrevious && v.ConfirmedHash != "" && hash == v.ConfirmedHash) {
		return errors.New("the provider ingress configuration changed outside Luci; no mutation was made")
	}
	return nil
}
func ownedDataService(v dataDomainRecord, observed coolifyService, allowPrevious bool) error {
	if err := ownedDataServiceConfiguration(v, observed, allowPrevious); err != nil {
		return err
	}
	hash, _ := dataComposeHash(observed.Compose)
	if hash == v.ComposeHash {
		if len(observed.Applications) != 1 || observed.Applications[0].Name != "data-ingress" || observed.Applications[0].FQDN != dataServiceURL(v) {
			return errors.New("the provider did not confirm this data domain and internal port")
		}
	}
	return nil
}
func (c *CoolifyClient) createDataService(ctx context.Context, v dataDomainRecord, compose string) (string, error) {
	body := map[string]any{"name": dataServiceName(v), "description": dataServiceMarker(v), "project_uuid": v.ProviderProjectUUID, "server_uuid": v.ServerUUID, "environment_name": v.Environment, "docker_compose_raw": base64.StdEncoding.EncodeToString([]byte(compose)), "urls": []map[string]string{{"name": "data-ingress", "url": dataServiceURL(v)}}, "instant_deploy": false, "force_domain_override": false}
	if v.DestinationUUID != "" {
		body["destination_uuid"] = v.DestinationUUID
	}
	var result struct {
		UUID string `json:"uuid"`
	}
	if err := c.request(ctx, http.MethodPost, "/services", nil, body, &result); err != nil {
		return "", err
	}
	if !coolifyID.MatchString(result.UUID) {
		return "", errors.New("Coolify did not confirm the new data service identifier")
	}
	return result.UUID, nil
}
func (c *CoolifyClient) updateDataService(ctx context.Context, v dataDomainRecord, compose string) error {
	return c.request(ctx, http.MethodPatch, "/services/"+v.ProviderServiceUUID, nil, map[string]any{"name": dataServiceName(v), "description": dataServiceMarker(v), "docker_compose_raw": base64.StdEncoding.EncodeToString([]byte(compose)), "urls": []map[string]string{{"name": "data-ingress", "url": dataServiceURL(v)}}, "instant_deploy": false, "force_domain_override": false}, nil)
}
func (c *CoolifyClient) activateDataService(ctx context.Context, id string, restart bool) error {
	action := "start"
	if restart {
		action = "restart"
	}
	return c.request(ctx, http.MethodPost, "/services/"+id+"/"+action, url.Values{"latest": {"false"}}, nil, nil)
}
func (c *CoolifyClient) stopDataService(ctx context.Context, id string) error {
	return c.request(ctx, http.MethodPost, "/services/"+id+"/stop", url.Values{"docker_cleanup": {"false"}}, nil, nil)
}

// A dedicated, unprivileged sidecar exposes only client APIs. Native admin,
// Studio, database ports and the control plane never join the public listener.
func dataIngressCompose(v dataDomainRecord, key string) (string, error) {
	policy := "local key=" + strconv.Quote(key) + "\nlocal app=" + strconv.Quote(v.AppOrigin) + "\nlocal api=" + strconv.Quote(v.Domain) + "\n" + dataIngressPolicy
	config := map[string]any{"static_resources": map[string]any{
		"listeners": []any{map[string]any{"name": "client", "address": map[string]any{"socket_address": map[string]any{"address": "0.0.0.0", "port_value": 8080}}, "filter_chains": []any{map[string]any{"filters": []any{map[string]any{"name": "envoy.filters.network.http_connection_manager", "typed_config": map[string]any{
			"@type": "type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager", "stat_prefix": "luci_client", "normalize_path": true, "merge_slashes": true, "path_with_escaped_slashes_action": "REJECT_REQUEST", "upgrade_configs": []any{map[string]any{"upgrade_type": "websocket"}},
			"route_config": map[string]any{"name": "client", "virtual_hosts": []any{map[string]any{"name": "client", "domains": []string{"*"}, "typed_per_filter_config": map[string]any{"envoy.filters.http.cors": map[string]any{"@type": "type.googleapis.com/envoy.extensions.filters.http.cors.v3.CorsPolicy", "allow_origin_string_match": []any{map[string]any{"exact": v.AppOrigin}, map[string]any{"exact": v.Domain}}, "allow_methods": "GET,HEAD,POST,PUT,PATCH,DELETE,OPTIONS", "allow_headers": "authorization,apikey,content-type,x-client-info,prefer,range,range-unit,x-upsert,tus-resumable,upload-offset,upload-length,upload-metadata", "expose_headers": "content-range,range-unit,upload-offset,tus-resumable", "max_age": "300"}}, "routes": []any{
				map[string]any{"match": map[string]any{"path": v.ProbePath}, "direct_response": map[string]any{"status": 200, "body": map[string]any{"inline_string": v.ConfigurationDigest}}},
				map[string]any{"match": map[string]any{"prefix": "/"}, "route": map[string]any{"cluster": "native_data", "timeout": "0s", "host_rewrite_literal": "supabase-api"}},
			}}}},
			// The policy never changes Host/path routing. Header sanitization must
			// retain the already selected route and its per-route CORS policy.
			"http_filters": []any{map[string]any{"name": "envoy.filters.http.lua", "typed_config": map[string]any{"@type": "type.googleapis.com/envoy.extensions.filters.http.lua.v3.Lua", "inline_code": policy, "clear_route_cache": false}}, map[string]any{"name": "envoy.filters.http.cors", "typed_config": map[string]any{"@type": "type.googleapis.com/envoy.extensions.filters.http.cors.v3.Cors"}}, map[string]any{"name": "envoy.filters.http.router", "typed_config": map[string]any{"@type": "type.googleapis.com/envoy.extensions.filters.http.router.v3.Router"}}},
		}}}}}}},
		"clusters": []any{map[string]any{"name": "native_data", "connect_timeout": "5s", "type": "STRICT_DNS", "dns_lookup_family": "V4_ONLY", "dns_refresh_rate": "1s", "respect_dns_ttl": false, "load_assignment": map[string]any{"cluster_name": "native_data", "endpoints": []any{map[string]any{"lb_endpoints": []any{map[string]any{"endpoint": map[string]any{"address": map[string]any{"socket_address": map[string]any{"address": "supabase-api", "port_value": 8000}}}}}}}}}},
	}}
	b, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	compose := map[string]any{"services": map[string]any{"data-ingress": map[string]any{
		"image": dataIngressImage, "user": "101:101", "command": []string{"envoy", "-c", "/etc/luci/ingress.json", "--disable-hot-restart", "--concurrency", "1"}, "restart": "unless-stopped", "read_only": true, "cap_drop": []string{"ALL"}, "security_opt": []string{"no-new-privileges:true"}, "mem_limit": "128m", "memswap_limit": "128m", "cpus": 0.25, "pids_limit": 256, "tmpfs": []string{"/tmp:rw,noexec,nosuid,size=16777216"}, "expose": []string{"8080"}, "configs": []any{map[string]any{"source": "luci-data-client", "target": "/etc/luci/ingress.json", "mode": 292}}, "networks": []string{"native-project"}, "labels": map[string]string{"ai.lcfr.luci.data-owner": v.Owner, "ai.lcfr.luci.project": v.ProjectID, "ai.lcfr.luci.data-tenant": v.StableTenant},
	}}, "configs": map[string]any{"luci-data-client": map[string]any{"content": string(b)}}, "networks": map[string]any{"native-project": map[string]any{"external": true, "name": v.ApplicationNetwork}}}
	b, err = json.Marshal(compose)
	return string(b), err
}

// Upstream Supabase still verifies JWT signatures and RLS. This policy excludes
// controller/admin credentials before they can reach an administrative route.
const dataIngressPolicy = `
local function decode64(s)
  if not s or #s==0 or s:find('[^%w_-]') then return nil end
  local alphabet='ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/'
  s=s:gsub('-','+'):gsub('_','/')
  local out,acc,bits={},0,0
  for i=1,#s do
    local n=alphabet:find(s:sub(i,i),1,true); if not n then return nil end
    acc=acc*64+n-1; bits=bits+6
    if bits>=8 then bits=bits-8; out[#out+1]=string.char(math.floor(acc/2^bits)); acc=acc%2^bits end
  end
  if bits>=6 or acc~=0 then return nil end
  return table.concat(out)
end
local function roleOfJSON(s)
  local p,role,count=1,nil,0
  local function space() while s:sub(p,p):match('%s') do p=p+1 end end
  local function str()
    if s:sub(p,p)~='"' then error('string') end; p=p+1; local out={}
    while p<=#s do
      local c=s:sub(p,p); p=p+1
      if c=='"' then return table.concat(out) end
      if c=='\\' then
        c=s:sub(p,p); p=p+1
        if c=='u' then local h=s:sub(p,p+3); if not h:match('^%x%x%x%x$') then error('escape') end; p=p+4; local n=tonumber(h,16); c=n<128 and string.char(n) or '?'
        elseif c=='"' or c=='\\' or c=='/' then
        elseif c=='b' or c=='f' or c=='n' or c=='r' or c=='t' then c='?'
        else error('escape') end
      elseif c:byte()<32 then error('control') end
      out[#out+1]=c
    end
    error('unterminated')
  end
  local value
  value=function(depth)
    if depth>32 then error('depth') end; space(); local c=s:sub(p,p)
    if c=='"' then return str() end
    if c=='{' or c=='[' then
      p=p+1; space(); local closing=c=='{' and '}' or ']'
      if s:sub(p,p)==closing then p=p+1; return false end
      while true do
        local name=nil
        if c=='{' then name=str(); space(); if s:sub(p,p)~=':' then error('colon') end; p=p+1 end
        local v=value(depth+1)
        if depth==0 and name=='role' then role=v; count=count+1 end
        space(); local next=s:sub(p,p); p=p+1
        if next==closing then break end; if next~=',' then error('comma') end; space()
      end
      return false
    end
    local literal=s:sub(p):match('^[^,%]}%s]+'); if not literal then error('literal') end
    if literal~='true' and literal~='false' and literal~='null' and not tonumber(literal) then error('literal') end
    p=p+#literal; return false
  end
  if s:sub(1,1)~='{' then return nil end
  local ok=pcall(function() value(0); space(); if p~=#s+1 then error('suffix') end end)
  if ok and count==1 and (role=='anon' or role=='authenticated') then return role end
  return nil
end
local function clientJWT(token)
  if not token or #token<32 or #token>4096 then return false end
  local a,b,c=token:match('^([%w_-]+)%.([%w_-]+)%.([%w_-]+)$')
  if not a or not decode64(a) or not decode64(c) then return false end
  local payload=decode64(b); return payload and roleOfJSON(payload)~=nil
end
local function unescape(s) return s:gsub('%%(%x%x)',function(h) return string.char(tonumber(h,16)) end) end
local function starts(path,base) return path==base or path:sub(1,#base+1)==base..'/' end
function envoy_on_request(h)
  local headers=h:headers(); local raw=headers:get(':path') or ''
  local path=raw:match('^[^?]*'); for _=1,3 do path=unescape(path) end
  if path:find('%%[%x][%x]') or path:find('%z') or path:find('\\',1,true) then h:respond({[':status']='404'},'Unavailable'); return end
  local origin=headers:get('origin')
  if origin and origin~=app and origin~=api then h:respond({[':status']='403'},'Application origin is not approved'); return end
  h:streamInfo():dynamicMetadata():set('luci.data','origin',origin or '')
  if raw:match('^[^?]*')=='/__luci/data-health' and headers:get(':method')=='GET' then return end
  local allowed=false
  for _,base in ipairs({'/auth/v1','/rest/v1','/storage/v1','/realtime/v1','/functions/v1','/graphql/v1'}) do if starts(path,base) then allowed=true end end
  if not allowed or starts(path,'/auth/v1/admin') or starts(path,'/realtime/v1/api') then h:respond({[':status']='404'},'Unavailable'); return end
  local authorization=headers:get('authorization')
  if authorization and (authorization:sub(1,7)~='Bearer ' or not clientJWT(authorization:sub(8))) then h:respond({[':status']='401'},'Use a Supabase client token'); return end
  local supplied=headers:get('apikey')
  if supplied and supplied~=key then h:respond({[':status']='401'},'Use this project publishable key'); return end
  local query=raw:match('%?(.*)$') or ''
  for name,value in query:gmatch('([^&=]+)=([^&]*)') do
    name=unescape(name); value=unescape(value)
    if (name=='apikey' and value~=key) or (name=='access_token' and not clientJWT(value)) then h:respond({[':status']='401'},'Use this project client credentials'); return end
  end
  local remove={}
  for name,_ in pairs(headers) do if name:sub(1,7)=='x-luci-' or name:sub(1,12)=='x-forwarded-' or name=='forwarded' or name=='proxy-authorization' then remove[#remove+1]=name end end
  for _,name in ipairs(remove) do headers:remove(name) end
end
function envoy_on_response(h)
  local headers=h:headers(); headers:remove('access-control-allow-origin'); headers:remove('access-control-allow-credentials')
  local meta=h:streamInfo():dynamicMetadata():get('luci.data'); local origin=meta and meta['origin']
  if origin and (origin==app or origin==api) then headers:replace('access-control-allow-origin',origin); headers:add('vary','Origin') end
  headers:replace('x-content-type-options','nosniff'); headers:replace('referrer-policy','no-referrer')
end
`

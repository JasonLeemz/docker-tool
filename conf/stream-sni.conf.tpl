{{- if .EnableSNI }}
# 基于域名的路由映射
map $ssl_preread_server_name $backend_pool {
{{- range $domain, $upstream := .DomainRoutes }}
    {{ $domain }}    {{ $upstream }};
{{- end }}
    default          {{ .DefaultRoute }};
}

{{- range $upstream, $servers := .StaticUpstreams }}
# {{ $upstream }} 后端
upstream {{ $upstream }} {
{{- range $servers }}
    server {{ . }};
{{- end }}
}
{{- end }}

{{- if .DefaultRoute }}
# 默认后端
upstream {{ .DefaultRoute }} {
{{- range .Upstream }}
    server {{ .IP }}:{{ .Port.Port }};
{{- end }}
}
{{- end }}

# Stream 服务器配置  
server {
    listen {{ .ListenPort }};
    ssl_preread on;
    proxy_pass $backend_pool;
    proxy_timeout 3s;
    proxy_connect_timeout 1s;
}
{{- else }}
# 传统 Stream 配置
upstream {{ .ServiceName }} {
{{- range .Upstream }}
    server {{ .IP }}:{{ .Port.Port }};
{{- end }}
}

server {
    listen {{ .ListenPort }};
    proxy_pass {{ .ServiceName }};
}
{{- end }}

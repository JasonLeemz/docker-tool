{{- if .EnableWebSocket }}
map $http_upgrade $connection_upgrade {
    default upgrade;
    '' close;
}
{{- end }}

upstream {{ .ServiceName }} {
{{- range .Upstream }}
    server {{ .IP }}:{{ .Port.Port }};
{{- end }}
}

{{- if .EnableSSL }}
{{- if .ForceHTTPS }}
# HTTP 到 HTTPS 重定向
server {
    listen          80;
    listen          [::]:80;

    server_name     {{ .Domain }};
    rewrite ^(.*)$  https://$host$1 permanent;
}
{{- end }}

# HTTPS 服务器配置
server {
    listen  443       ssl;
    listen  [::]:443  ssl;
    http2   on;

    server_name {{ .Domain }};

    {{- if .SSLCertificate }}
    ssl_certificate     {{ .SSLCertificate }};
    {{- end }}
    {{- if .SSLCertificateKey }}
    ssl_certificate_key {{ .SSLCertificateKey }};
    {{- end }}

    location {{ .Path }} {
        {{- if .EnableWebSocket }}
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        {{- end }}

        {{- if .ClientMaxBodySize }}
        client_max_body_size {{ .ClientMaxBodySize }};
        {{- end }}

        proxy_pass http://{{ .ServiceName }}/;

        {{- if .ProxyHTTPVersion }}
        proxy_http_version {{ .ProxyHTTPVersion }};
        {{- end }}

        {{- range .ProxyHeaders }}
        proxy_set_header {{ . }};
        {{- end }}

        {{- if .ProxyRedirect }}
        proxy_redirect {{ .ProxyRedirect }};
        {{- end }}
    }
}
{{- else }}
# HTTP 服务器配置
server {
    listen 80;
    server_name {{ .Domain }};

    location {{ .Path }} {
        {{- if .EnableWebSocket }}
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        {{- end }}

        {{- if .ClientMaxBodySize }}
        client_max_body_size {{ .ClientMaxBodySize }};
        {{- end }}

        proxy_pass http://{{ .ServiceName }}/;

        {{- if .ProxyHTTPVersion }}
        proxy_http_version {{ .ProxyHTTPVersion }};
        {{- end }}

        {{- range .ProxyHeaders }}
        proxy_set_header {{ . }};
        {{- end }}

        {{- if .ProxyRedirect }}
        proxy_redirect {{ .ProxyRedirect }};
        {{- end }}
    }
}
{{- end }}

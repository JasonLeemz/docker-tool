upstream {{ .ServiceName }} {
{{- range .Upstream }}
    server {{ .IP }}:{{ .Port.Port }};
{{- end }}
}

server {
    listen {{ .ListenPort }};
    proxy_pass {{ .ServiceName }};
}

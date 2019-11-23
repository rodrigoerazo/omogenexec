{{ define "helper_contest_banner" }}
    {{ if . }}
        <header class="article-header">
            <div class="row">
                {{ if .Started }}
                    <h1 class="display">Tävlingen slutar om {{ .UntilEnd | interval }}</h1>
                    {{ else  }}
                    <h1 class="display">Tävlingen börjar om {{ .UntilStart | interval }}</h1>
                {{ end }}
            </div>
        </header>
    {{ end }}
{{ end }}

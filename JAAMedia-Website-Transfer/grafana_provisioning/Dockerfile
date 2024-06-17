FROM grafana/grafana
COPY *.json /etc/grafana/provisioning/dashboards/
COPY influx.yaml /etc/grafana/provisioning/datasources/influx.yml
COPY pg.yaml /etc/grafana/provisioning/datasources/pg.yml
COPY shopify.yaml /etc/grafana/provisioning/datasources/shopify.yml

COPY dashboards.yaml /etc/grafana/provisioning/dashboards/dashboards.yml
ENV GF_INSTALL_PLUGINS=https://grafana.com/api/plugins/marcusolsson-json-datasource/versions/1.3.6/download;marcusolsson-json-datasource
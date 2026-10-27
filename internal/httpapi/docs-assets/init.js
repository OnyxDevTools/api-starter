// internal/httpapi/docs-assets/init.js
window.addEventListener("DOMContentLoaded", function () {
  window.ui = SwaggerUIBundle({
    url: "/openapi.json",
    dom_id: "#swagger-ui",
    presets: [SwaggerUIBundle.presets.apis],
    layout: "BaseLayout",
    validatorUrl: null,
    queryConfigEnabled: false,
    persistAuthorization: false,
    deepLinking: true,
    filter: true,
    displayRequestDuration: true,
    defaultModelsExpandDepth: -1,
    docExpansion: "list",
    supportedSubmitMethods: ["get", "post", "put", "delete"],
  });
});

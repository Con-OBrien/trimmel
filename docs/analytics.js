// Cookie-free site analytics (Umami). Inactive until an ID is set below.
// Create a free website at https://cloud.umami.is, copy its Website ID here, and
// update the "Website analytics" section of privacy.html before publishing.
(function () {
  var id = "";
  if (!id) return;
  var s = document.createElement("script");
  s.defer = true;
  s.src = "https://cloud.umami.is/script.js";
  s.setAttribute("data-website-id", id);
  s.setAttribute("data-do-not-track", "true");
  document.head.appendChild(s);
})();

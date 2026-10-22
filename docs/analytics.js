// Cookie-free site analytics (Umami). Does nothing if the ID below is empty.
(function () {
  var id = "2214789b-0b5e-4bfa-adf1-86422daa9fa8";
  if (!id) return;
  var s = document.createElement("script");
  s.defer = true;
  s.src = "https://cloud.umami.is/script.js";
  s.setAttribute("data-website-id", id);
  s.setAttribute("data-do-not-track", "true");
  document.head.appendChild(s);
})();

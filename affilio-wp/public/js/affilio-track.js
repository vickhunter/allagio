document.addEventListener('click', function (e) {
  const link = e.target.closest('a[data-affilio-track]');
  if (!link) return;
  const payload = {
    recommendation_id: link.dataset.affilioTrack,
    article_id: link.dataset.affilioArticle,
    program_id: link.dataset.affilioProgram,
  };
  navigator.sendBeacon('/wp-json/affilio/v1/track', JSON.stringify(payload));
});

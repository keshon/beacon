(function () {
    function currentTheme() {
        return document.documentElement.getAttribute('data-bs-theme') || 'dark';
    }
    function syncThemeToggle() {
        var t = currentTheme();
        var btn = document.getElementById('beaconThemeToggle');
        if (!btn) return;
        btn.setAttribute('aria-pressed', t === 'dark' ? 'true' : 'false');
        btn.title = t === 'dark' ? 'Switch to light theme' : 'Switch to dark theme';
        var sun = btn.querySelector('.theme-icon-sun');
        var moon = btn.querySelector('.theme-icon-moon');
        if (sun && moon) {
            sun.classList.toggle('d-none', t !== 'dark');
            moon.classList.toggle('d-none', t === 'dark');
        }
    }
    document.getElementById('beaconThemeToggle')?.addEventListener('click', function () {
        var next = currentTheme() === 'dark' ? 'light' : 'dark';
        document.documentElement.setAttribute('data-bs-theme', next);
        try {
            localStorage.setItem('beaconTheme', next);
        } catch (e) {}
        syncThemeToggle();
    });
    document.addEventListener('DOMContentLoaded', syncThemeToggle);
})();

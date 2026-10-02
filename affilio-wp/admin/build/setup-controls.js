(() => {
    'use strict';
    document.documentElement.classList.add('copy-ready');
    document.addEventListener('click', async event => {
        const button = event.target.closest('[data-copy]');
        if (!button || button.disabled) return;
        const field = button.closest('.copy-field');
        const content = field?.querySelector('textarea, pre');
        const status = field?.querySelector('.copy-status');
        if (!content || !status) return;
        button.disabled = true;
        status.textContent = '';
        try {
            await navigator.clipboard.writeText(content.value ?? content.textContent);
            status.textContent = 'Copied. Paste it into your terminal.';
        } catch {
            if (content instanceof HTMLTextAreaElement) {
                content.focus();
                content.select();
            } else {
                const range = document.createRange();
                range.selectNodeContents(content);
                const selection = window.getSelection();
                selection.removeAllRanges();
                selection.addRange(range);
            }
            status.textContent = 'Select and copy the command below.';
        } finally {
            button.disabled = false;
        }
    });
})();

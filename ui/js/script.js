document.addEventListener('DOMContentLoaded', () => {
    const updatesContainer = document.getElementById('updates-container');

    // Helper function to add a message to the container
    function addMessage(message, className = '') {
        const p = document.createElement('p');
        p.className = `story-item ${className}`;
        p.textContent = message;
        updatesContainer.prepend(p);
        // Optional: Limit the number of displayed items
        while (updatesContainer.children.length > 20) {
            updatesContainer.lastChild.remove();
        }
    }

    // Create a new EventSource object, pointing to our SSE endpoint
    const eventSource = new EventSource('/hn-events');

    // Event listener for when the connection opens
    eventSource.onopen = function(event) {
        console.log('SSE connection to Hacker News stream opened.');
        if (updatesContainer.querySelector('.initial-load')) {
            updatesContainer.innerHTML = ''; // Clear initial message
        }
        addMessage('Connection established. Waiting for new stories...', 'info-message');
    };

    // Event listener for the custom 'new-story' event
    eventSource.addEventListener('new-story', function(event) {
        try {
            const story = JSON.parse(event.data);
            console.log('Received new story:', story);

            const storyElement = document.createElement('div');
            storyElement.className = 'story-item';
            storyElement.innerHTML = `
                <a href="${story.url || '#'}" target="_blank" rel="noopener noreferrer">
                    ${story.title || 'No Title'}
                </a>
                <span class="meta-info">by ${story.by || 'N/A'} | score: ${story.score || 0}</span>
            `;
            updatesContainer.prepend(storyElement);

            // Optional: Limit the number of displayed stories to keep the UI clean
            while (updatesContainer.children.length > 20) {
                updatesContainer.lastChild.remove();
            }

        } catch (e) {
            console.error('Error parsing story data:', e, event.data);
            addMessage(`Error processing story: ${e.message}`, 'error-message');
        }
    });

    // Event listener for 'no-new-data' events from the server
    eventSource.addEventListener('no-new-data', function(event) {
        console.log('Received no-new-data event:', event.data);
    });

    // Event listener for generic 'error' events
    eventSource.addEventListener('error', function(event) {
        console.error('Received error event:', event.data || 'Unknown error');
        addMessage(`Server Error: ${event.data || 'Unknown'}. See console.`, 'error-message');
    });

    // Event listener for 'story-error' events
    eventSource.addEventListener('story-error', function(event) {
        console.error('Received story-specific error event:', event.data);
        addMessage(`Story Error: ${event.data}.`, 'error-message');
    });

    // Generic onerror handler for EventSource connection errors
    eventSource.onerror = function(error) {
        console.error('EventSource connection error:', error);
        addMessage('Connection lost! Attempting to reconnect...', 'error-message');
    };

    // Event listener for the custom 'connected' event
    eventSource.addEventListener('connected', function(event) {
        console.log('Server reports connection established:', event.data);
    });
});
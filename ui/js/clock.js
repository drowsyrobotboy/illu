function updateClock() {
    const now = new Date();
    
    // Get time components
    let hours = now.getHours();
    const minutes = now.getMinutes();
    const seconds = now.getSeconds();
    
    // Format time with leading zeros
    const formattedHours = hours.toString().padStart(2, '0');
    const formattedMinutes = minutes.toString().padStart(2, '0');
    const formattedSeconds = seconds.toString().padStart(2, '0');
    
    // Create time string with colored components
    let timeString = `<span class="hours">${formattedHours}</span>`;
    timeString += `<span class="separator">:</span>`;
    timeString += `<span class="minutes">${formattedMinutes}</span>`;
    timeString += `<span class="separator">:</span>`;
    timeString += `<span class="seconds">${formattedSeconds}</span>`;
    
    // Update time display
    document.getElementById('timeDisplay').innerHTML = timeString;
    
    // Update date
    const dateOptions = { 
        year: 'numeric', 
        month: 'long', 
        day: 'numeric' 
    };
    const dateString = now.toLocaleDateString('en-US', dateOptions);
    document.getElementById('dateDisplay').textContent = dateString;
    
    // Update day
    const dayOptions = { weekday: 'long' };
    const dayString = now.toLocaleDateString('en-US', dayOptions);
    document.getElementById('dayDisplay').textContent = dayString;
}

// Initialize clock
updateClock();

// Update every second
setInterval(updateClock, 1000);
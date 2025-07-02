const statsSource = new EventSource("/stats");

statsSource.onmessage = function(event) {
    const data = JSON.parse(event.data);
// CPU
    document.getElementById("cpu_percent").textContent = data.cpu_percent.toFixed(2);
    document.getElementById("cpu_cores").textContent = data.cpu_cores;

    // Load Average
    document.getElementById("load1").textContent = data.load1.toFixed(2);
    document.getElementById("load5").textContent = data.load5.toFixed(2);
    document.getElementById("load15").textContent = data.load15.toFixed(2);

    // Memory
    document.getElementById("mem_used").textContent = data.mem_used;
    document.getElementById("mem_total").textContent = data.mem_total;
    document.getElementById("mem_percent").textContent = data.mem_percent.toFixed(1);

    // Swap
    document.getElementById("swap_used").textContent = data.swap_used;
    document.getElementById("swap_total").textContent = data.swap_total;
    document.getElementById("swap_percent").textContent = data.swap_percent.toFixed(1);

    // Temperature
    document.getElementById("temp").textContent = data.temp.toFixed(1);

    // Disk
    document.getElementById("disk_used").textContent = data.disk_used;
    document.getElementById("disk_total").textContent = data.disk_total;
    document.getElementById("disk_percent").textContent = data.disk_percent.toFixed(1);
};
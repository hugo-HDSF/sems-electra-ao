// Initialize Swagger UI
window.onload = () => {
    window.ui = SwaggerUIBundle({
        url: "/swagger/openapi.yaml",
        dom_id: '#swagger-ui',
        docExpansion: 'list',
        defaultModelsExpandDepth: -1, // Hide models section for cleaner UI
        deepLinking: true,
        presets: [
            SwaggerUIBundle.presets.apis,
            SwaggerUIBundle.SwaggerUIStandalonePreset
        ],
    });
};

function updateDashboard(data) {
    if (!data) return;
    
    // Update KPIs
    document.getElementById('gridLimit').innerText = data.GridLimitKW.toFixed(0);
    document.getElementById('totalAllocated').innerText = data.TotalAllocatedKW.toFixed(2);
    document.getElementById('availablePower').innerText = data.AvailablePowerKW.toFixed(2) + " kW";
    
    if (data.BESS) {
        document.getElementById('bessSoC').innerText = data.BESS.SoCPercent;
        
        let flow = data.BESS.CurrentPowerKW;
        document.getElementById('bessPower').innerText = (flow > 0 ? "+" : "") + flow.toFixed(2) + " kW";
        
        const badge = document.getElementById('bessStatusBadge');
        badge.innerText = data.BESS.Status;
        
        if (data.BESS.Status === 'charging') {
            badge.className = "text-xs font-bold px-3 py-1 rounded-full bg-emerald-500/20 text-emerald-400 border border-emerald-500/30 uppercase";
        } else if (data.BESS.Status === 'discharging') {
            badge.className = "text-xs font-bold px-3 py-1 rounded-full bg-rose-500/20 text-rose-400 border border-rose-500/30 uppercase";
        } else {
            badge.className = "text-xs font-bold px-3 py-1 rounded-full bg-slate-500/20 text-slate-300 border border-slate-500/30 uppercase";
        }
    }

        // Update Table
        const tbody = document.getElementById('sessionsTableBody');
        if (!data.EVSEs || data.EVSEs.length === 0) {
            tbody.innerHTML = '<tr><td colspan="3" class="py-8 text-center text-slate-500 italic">No connectors found</td></tr>';
            return;
        }

        let html = '';
        data.EVSEs.forEach(evse => {
            html += `<tr class="bg-slate-800/80"><td colspan="3" class="py-2 px-4 font-bold text-slate-400 text-xs uppercase tracking-wider">${evse.ID} <span class="font-normal text-slate-500 ml-2">(Max: ${evse.MaxPowerKW} kW)</span></td></tr>`;
            
            evse.Connectors.forEach(conn => {
                if (conn.Session) {
                    html += `
                    <tr class="hover:bg-slate-800/50 transition-colors">
                        <td class="py-4 px-4 font-medium text-white">${conn.ID} <span class="text-[10px] font-bold uppercase px-2 py-0.5 rounded-full bg-emerald-500/20 text-emerald-400 ml-2">Charging</span></td>
                        <td class="py-4 px-4">
                            <div class="flex items-center gap-3">
                                <span class="w-12 font-medium">${conn.Session.EVSoCPercent}</span>
                                <div class="w-24 bg-slate-700 h-2 rounded-full overflow-hidden">
                                    <div class="bg-blue-500 h-full rounded-full transition-all duration-500" style="width: ${conn.Session.EVSoC * 100}%"></div>
                                </div>
                            </div>
                        </td>
                        <td class="py-4 px-4 text-right font-medium text-sky-400">${conn.Session.AllocatedPowerKW.toFixed(2)} kW</td>
                    </tr>
                    `;
                } else {
                    html += `
                    <tr class="hover:bg-slate-800/50 transition-colors opacity-60">
                        <td class="py-4 px-4 font-medium text-slate-400">${conn.ID} <span class="text-[10px] font-bold uppercase px-2 py-0.5 rounded-full bg-slate-500/20 text-slate-400 ml-2">Available</span></td>
                        <td class="py-4 px-4 text-slate-500">---</td>
                        <td class="py-4 px-4 text-right text-slate-500">0.00 kW</td>
                    </tr>
                    `;
                }
            });
        });
        tbody.innerHTML = html;
}

async function tickSimulation() {
    try {
        await fetch('/api/v1/simulate/tick', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ durationMinutes: 1 })
        });
        // SSE will automatically push the new state
    } catch (err) {
        console.error("Failed to tick simulation", err);
    }
}

// Subscribe to SSE stream for real-time updates
const evtSource = new EventSource('/api/v1/status/stream');

evtSource.onmessage = (event) => {
    try {
        const data = JSON.parse(event.data);
        updateDashboard(data);
    } catch (err) {
        console.error("Failed to parse SSE data", err);
    }
};

evtSource.onerror = (err) => {
    console.error("SSE connection error", err);
};

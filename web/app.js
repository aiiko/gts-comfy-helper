const promptEl = document.getElementById('prompt')
const generateBtn = document.getElementById('generateBtn')
const previewImageEl = document.getElementById('previewImage')
const previewPlaceholderEl = document.getElementById('previewPlaceholder')
const statusPillEl = document.getElementById('statusPill')
const metaLineEl = document.getElementById('metaLine')
const positiveTagsEl = document.getElementById('positiveTags')
const negativeTagsEl = document.getElementById('negativeTags')
const saveSettingsBtn = document.getElementById('saveSettingsBtn')
const saveStatusEl = document.getElementById('saveStatus')

let activeJobID = ''
let previewSeq = 0
let previewTimer = null
let jobTimer = null

function setPill(kind, text) {
  statusPillEl.className = `pill ${kind}`
  statusPillEl.textContent = text
}

function setPreviewImage(src) {
  if (!src) return
  previewImageEl.src = src
  previewImageEl.style.display = 'block'
  previewPlaceholderEl.style.display = 'none'
}

function resetPreview() {
  previewImageEl.removeAttribute('src')
  previewImageEl.style.display = 'none'
  previewPlaceholderEl.style.display = 'grid'
  previewPlaceholderEl.textContent = 'No hay generación activa'
  metaLineEl.textContent = ''
  setPill('waiting', 'Esperando')
}

async function requestJSON(path, options = {}) {
  const response = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  const data = await response.json().catch(() => ({}))
  if (!response.ok) {
    const message = data?.error?.message || `Request failed (${response.status})`
    throw new Error(message)
  }
  return data
}

async function loadSettings() {
  try {
    const settings = await requestJSON('/api/settings')
    positiveTagsEl.value = settings.positive_tags || ''
    negativeTagsEl.value = settings.negative_tags || ''
    saveStatusEl.textContent = 'Configuración cargada.'
  } catch (error) {
    saveStatusEl.textContent = `Error cargando configuración: ${error.message}`
  }
}

async function saveSettings() {
  saveSettingsBtn.disabled = true
  saveStatusEl.textContent = 'Guardando...'
  try {
    const body = {
      positive_tags: positiveTagsEl.value.trim(),
      negative_tags: negativeTagsEl.value.trim(),
    }
    await requestJSON('/api/settings', {
      method: 'PUT',
      body: JSON.stringify(body),
    })
    saveStatusEl.textContent = 'Configuración guardada.'
  } catch (error) {
    saveStatusEl.textContent = `Error guardando configuración: ${error.message}`
  } finally {
    saveSettingsBtn.disabled = false
  }
}

function stopPolling() {
  if (previewTimer) {
    clearTimeout(previewTimer)
    previewTimer = null
  }
  if (jobTimer) {
    clearTimeout(jobTimer)
    jobTimer = null
  }
}

async function pollPreview() {
  if (!activeJobID) return
  try {
    const payload = await requestJSON(`/api/jobs/${encodeURIComponent(activeJobID)}/preview?since_seq=${encodeURIComponent(String(previewSeq))}`)
    const nextSeq = Number.isFinite(payload.seq) ? Math.max(0, Math.floor(payload.seq)) : previewSeq
    previewSeq = Math.max(previewSeq, nextSeq)

    const previewStatus = String(payload.preview_status || 'waiting').toLowerCase()
    if (previewStatus === 'failed') {
      setPill('failed', 'Error')
    } else if (previewStatus === 'done') {
      setPill('done', 'Finalizado')
    } else {
      setPill('running', 'Generando')
    }

    if (payload.warning) {
      metaLineEl.textContent = String(payload.warning)
    }

    if (payload.frame && payload.frame.data_base64) {
      const mime = (payload.frame.mime || 'image/png').trim()
      setPreviewImage(`data:${mime};base64,${payload.frame.data_base64}`)
    }
  } catch (error) {
    metaLineEl.textContent = `Error preview: ${error.message}`
  } finally {
    previewTimer = setTimeout(pollPreview, 500)
  }
}

async function pollJobStatus() {
  if (!activeJobID) return
  try {
    const job = await requestJSON(`/api/jobs/${encodeURIComponent(activeJobID)}`)
    const status = String(job.status || '').toLowerCase()
    if (status === 'done') {
      setPill('done', 'Finalizado')
      if (job.asset_url) {
        setPreviewImage(job.asset_url)
      }
      metaLineEl.textContent = 'Imagen final generada.'
      stopPolling()
      return
    }
    if (status === 'failed') {
      setPill('failed', 'Error')
      metaLineEl.textContent = job.error || 'La generación falló.'
      stopPolling()
      return
    }
    setPill('running', 'Generando')
  } catch (error) {
    metaLineEl.textContent = `Error estado job: ${error.message}`
  } finally {
    jobTimer = setTimeout(pollJobStatus, 1000)
  }
}

async function generate() {
  const prompt = promptEl.value.trim()
  if (!prompt) {
    metaLineEl.textContent = 'El prompt es obligatorio.'
    return
  }
  generateBtn.disabled = true
  stopPolling()
  previewSeq = 0
  resetPreview()
  setPill('running', 'Generando')
  previewPlaceholderEl.textContent = 'Esperando primeros frames...'
  metaLineEl.textContent = ''

  try {
    const result = await requestJSON('/api/generate', {
      method: 'POST',
      body: JSON.stringify({ prompt }),
    })
    activeJobID = String(result.job_id || '').trim()
    if (!activeJobID) throw new Error('No se recibió job_id')
    pollPreview()
    pollJobStatus()
  } catch (error) {
    setPill('failed', 'Error')
    metaLineEl.textContent = `Error generando: ${error.message}`
  } finally {
    generateBtn.disabled = false
  }
}

generateBtn.addEventListener('click', () => {
  void generate()
})
saveSettingsBtn.addEventListener('click', () => {
  void saveSettings()
})
positiveTagsEl.addEventListener('blur', () => {
  void saveSettings()
})
negativeTagsEl.addEventListener('blur', () => {
  void saveSettings()
})

resetPreview()
void loadSettings()

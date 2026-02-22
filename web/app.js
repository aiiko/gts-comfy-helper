const promptEl = document.getElementById('prompt')
const generateBtn = document.getElementById('generateBtn')
const aspectRatioEls = document.querySelectorAll('input[name="aspectRatio"]')
const previewImageEl = document.getElementById('previewImage')
const previewPlaceholderEl = document.getElementById('previewPlaceholder')
const imageModalEl = document.getElementById('imageModal')
const imageModalImgEl = document.getElementById('imageModalImg')
const imageModalCloseEl = document.getElementById('imageModalClose')
const statusPillEl = document.getElementById('statusPill')
const metaLineEl = document.getElementById('metaLine')
const positiveTagsEl = document.getElementById('positiveTags')
const negativeTagsEl = document.getElementById('negativeTags')
const artStyleSelectorEl = document.getElementById('artStyleSelector')
const bodyFramingSelectorEl = document.getElementById('bodyFramingSelector')
const cameraSelectorEl = document.getElementById('cameraSelector')
const giantessCountEl = document.getElementById('giantessCount')
const tiniesModeEl = document.getElementById('tiniesMode')
const tinyCountWrapEl = document.getElementById('tinyCountWrap')
const tinyCountEl = document.getElementById('tinyCount')
const tinyGenderWrapEl = document.getElementById('tinyGenderWrap')
const tinyGenderEl = document.getElementById('tinyGender')
const tinyDescriptorEl = document.getElementById('tinyDescriptor')
const saveSettingsBtn = document.getElementById('saveSettingsBtn')
const saveStatusEl = document.getElementById('saveStatus')

let activeJobID = ''
let activeJobAspectRatio = ''
let previewSeq = 0
let previewTimer = null
let jobTimer = null
const lastAspectStorageKey = 'gts_last_aspect_ratio'
const aspectSizeMap = {
  portrait: { width: 896, height: 1152 },
  square: { width: 1024, height: 1024 },
  landscape: { width: 1152, height: 896 },
}

function selectedAspectRatio() {
  for (const optionEl of aspectRatioEls) {
    if (optionEl.checked) return optionEl.value
  }
  return 'square'
}

function selectedCameraSelector() {
  if (!cameraSelectorEl) return ''
  return String(cameraSelectorEl.value || '').trim()
}

function selectedArtStyle() {
  if (!artStyleSelectorEl) return ''
  return String(artStyleSelectorEl.value || '').trim()
}

function selectedBodyFraming() {
  if (!bodyFramingSelectorEl) return ''
  return String(bodyFramingSelectorEl.value || '').trim()
}

function selectedGiantessCount() {
  if (!giantessCountEl) return 1
  const value = Number.parseInt(String(giantessCountEl.value || '').trim(), 10)
  return value === 2 ? 2 : 1
}

function selectedTiniesMode() {
  if (!tiniesModeEl) return 'count'
  const mode = String(tiniesModeEl.value || '').trim().toLowerCase()
  return mode === 'group' ? 'group' : 'count'
}

function selectedTinyCount() {
  if (!tinyCountEl) return 1
  const value = Number.parseInt(String(tinyCountEl.value || '').trim(), 10)
  if (!Number.isFinite(value) || value <= 0) return 1
  return value
}

function selectedTinyGender() {
  if (!tinyGenderEl) return ''
  return String(tinyGenderEl.value || '').trim()
}

function selectedTinyDescriptor() {
  if (!tinyDescriptorEl) return ''
  return String(tinyDescriptorEl.value || '').trim()
}

function syncTiniesModeUI() {
  const isGroup = selectedTiniesMode() === 'group'
  if (tinyCountWrapEl) {
    tinyCountWrapEl.classList.toggle('is-hidden', isGroup)
  }
  if (tinyGenderWrapEl) {
    tinyGenderWrapEl.classList.toggle('is-hidden', isGroup)
  }
  if (tinyCountEl) {
    tinyCountEl.disabled = isGroup
  }
  if (tinyGenderEl) {
    tinyGenderEl.disabled = isGroup
  }
}

function setAspectRatio(value) {
  const normalized = String(value || '').trim().toLowerCase()
  const target = aspectSizeMap[normalized] ? normalized : 'square'
  for (const optionEl of aspectRatioEls) {
    optionEl.checked = optionEl.value === target
  }
}

function loadAspectRatioFromStorage() {
  try {
    const stored = String(localStorage.getItem(lastAspectStorageKey) || '').trim().toLowerCase()
    return aspectSizeMap[stored] ? stored : ''
  } catch {
    return ''
  }
}

function saveAspectRatioToStorage(value) {
  const normalized = String(value || '').trim().toLowerCase()
  if (!aspectSizeMap[normalized]) return
  try {
    localStorage.setItem(lastAspectStorageKey, normalized)
  } catch {
    // Ignore browser storage errors.
  }
}

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
  closeImageModal()
  previewImageEl.removeAttribute('src')
  previewImageEl.style.display = 'none'
  previewPlaceholderEl.style.display = 'grid'
  previewPlaceholderEl.textContent = 'No hay generación activa'
  metaLineEl.textContent = ''
  setPill('waiting', 'Esperando')
}

function openImageModal(src) {
  const imageSrc = String(src || '').trim()
  if (!imageSrc) return
  imageModalImgEl.src = imageSrc
  imageModalEl.classList.add('open')
  imageModalEl.setAttribute('aria-hidden', 'false')
  document.body.style.overflow = 'hidden'
}

function closeImageModal() {
  imageModalEl.classList.remove('open')
  imageModalEl.setAttribute('aria-hidden', 'true')
  imageModalImgEl.removeAttribute('src')
  document.body.style.overflow = ''
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
    const fromAPI = String(settings.last_aspect_ratio || '').trim().toLowerCase()
    const fromStorage = loadAspectRatioFromStorage()
    setAspectRatio(fromStorage || fromAPI)
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

async function saveLastGenerationSettings(aspectRatio) {
  const normalized = String(aspectRatio || '').trim().toLowerCase()
  if (!aspectSizeMap[normalized]) return
  saveAspectRatioToStorage(normalized)
  try {
    await requestJSON('/api/settings', {
      method: 'PUT',
      body: JSON.stringify({ last_aspect_ratio: normalized }),
    })
  } catch (error) {
    metaLineEl.textContent = `Error guardando aspect ratio: ${error.message}`
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
      void saveLastGenerationSettings(activeJobAspectRatio || selectedAspectRatio())
      activeJobAspectRatio = ''
      metaLineEl.textContent = 'Imagen final generada.'
      stopPolling()
      return
    }
    if (status === 'failed') {
      activeJobAspectRatio = ''
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
  const aspectRatio = selectedAspectRatio()
  const size = aspectSizeMap[aspectRatio] || aspectSizeMap.square
  activeJobAspectRatio = aspectRatio
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
      body: JSON.stringify({
        prompt,
        giantess_count: selectedGiantessCount(),
        tinies_mode: selectedTiniesMode(),
        tiny_count: selectedTinyCount(),
        tiny_gender: selectedTinyGender(),
        tiny_descriptor: selectedTinyDescriptor(),
        art_style: selectedArtStyle(),
        body_framing: selectedBodyFraming(),
        camera_selector: selectedCameraSelector(),
        aspect_ratio: aspectRatio,
        width: size.width,
        height: size.height,
      }),
    })
    activeJobID = String(result.job_id || '').trim()
    if (!activeJobID) throw new Error('No se recibió job_id')
    pollPreview()
    pollJobStatus()
  } catch (error) {
    activeJobAspectRatio = ''
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
previewImageEl.addEventListener('click', () => {
  if (previewImageEl.style.display === 'none') return
  openImageModal(previewImageEl.src)
})
imageModalCloseEl.addEventListener('click', () => {
  closeImageModal()
})
imageModalEl.addEventListener('click', (event) => {
  if (event.target === imageModalEl) {
    closeImageModal()
  }
})
if (tiniesModeEl) {
  tiniesModeEl.addEventListener('change', () => {
    syncTiniesModeUI()
  })
}
document.addEventListener('keydown', (event) => {
  if (event.key === 'Escape' && imageModalEl.classList.contains('open')) {
    closeImageModal()
  }
})

resetPreview()
syncTiniesModeUI()
void (async () => {
  try {
    await loadSettings()
  } catch (error) {
    saveStatusEl.textContent = `Error cargando configuración: ${error.message}`
  }
})()
